// src/App.vue
<script setup>
import { ref, computed } from 'vue'
import axios from 'axios'
import './style.css'

const config = {
  network: 'Undernet',
  glinelookup_url: '/api2/glinelookup/:network/:ip',
  api_key: import.meta.env.VITE_API_KEY
}

const myip = ref('')
const ip = ref('')
const errormsg = ref('')
const glines = ref([])

const showRequestForm = ref(false)
const nickname = ref('')
const realname = ref('')
const email = ref('')
const message = ref('')

const isSubmitDisabled = computed(() => {
  return !nickname.value || !realname.value || !email.value || !message.value
})

const removalResponse = ref([])

// Try to get user's IP address
const getUserIP = async () => {
  try {
    const response = await axios.get('https://api.ipify.org?format=json')
    myip.value = response.data.ip
    ip.value = response.data.ip
  } catch (error) {
    console.error('Failed to get user IP:', error)
  }
}

const lookupGline = async () => {
  if (!ip.value) return
  
  errormsg.value = ''
  removalResponse.value = ''
  glines.value = []
  isSubmitDisabled.value = false
  showRequestForm.value = false
  const url = config.glinelookup_url
    .replace(':network', config.network.toLowerCase())
    .replace(':ip', ip.value)
    
  try {
    const response = await axios.get(url, {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${config.api_key}`
      }
    })
    glines.value = response.data

    if (Array.isArray(response.data) && response.data.length === 0) {
      errormsg.value = 'No Glines found'
    }

  } catch (error) {
    if (error.response && error.response.status === 400) {
      errormsg.value = 'Invalid IP address'
      return
    }
    errormsg.value = 'API call error ' + error.response.status + ': ' + error.response.data
    //errormsg.value = 'Failed to lookup Gline: ' + error.message
    console.error('Failed to lookup Gline:', error)
  }
}

const requestRemoval = async () => {
  const requestData = {
    network: config.network,
    ip: ip.value,
    nickname: nickname.value,
    realname: realname.value,
    email: email.value,
    message: message.value
  }

  try {
    const response = await axios.post('/api/requestrem', requestData, {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${config.api_key}`
      }
    })
    removalResponse.value = response.data
    console.log('Removal request response:', response.data)
  } catch (error) {
    console.error('Failed to request removal:', error)
  }
}

const handleKeyPress = (event) => {
  if (event.key === 'Enter') {
    lookupGline()
  }
}

getUserIP()
</script>

<template>
  <div class="container mx-auto px-4 py-8">
    <h1>Gline Lookup</h1>
    <!--p>Your IP: {{ myip }}</p-->
    <div class="input-container">
      <label class="label">IP address:</label>
      <input 
        type="text"
        v-model="ip"
        class="input"
        @keypress="handleKeyPress"
      >
      <button 
        @click="lookupGline"
        class="button"
      >
        Lookup Gline
      </button>
    </div>
    
    <p v-if="errormsg" class="error">{{ errormsg }}</p>
    
    <div v-if="glines.length > 0" class="table-container">
      <table class="table-auto">
        <thead>
          <tr>
            <th class="table-header">Mask</th>
            <th class="table-header">Reason</th>
            <th class="table-header">Status</th>
            <th class="table-header">Expiration Date</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="gline in glines" :key="gline.mask">
            <td class="table-cell">{{ gline.mask }}</td>
            <td class="table-cell">{{ formatReason(gline.reason) }}</td>
            <td class="table-cell">
              {{ gline.expirets * 1000 > Date.now() 
                ? `Expires in ${formatDuration(gline.expirets)}`
                : `Expired ${formatDuration(gline.expirets)} ago`
              }}
            </td>
            <td class="table-cell">{{ formatDate(gline.expirets) }}</td>
          </tr>
        </tbody>
      </table>
      <button 
        v-if="!showRequestForm"
        @click="showRequestForm = true"
        class="button mt-4"
      >
        Request removal
      </button>
      <div v-if="showRequestForm" class="request-form mt-4">
        <div class="input-container">
          <label class="label">Nickname:</label>
          <input type="text" v-model="nickname" class="input">
        </div>
        <div class="input-container">
          <label class="label">Real Name:</label>
          <input type="text" v-model="realname" class="input">
        </div>
        <div class="input-container">
          <label class="label">Email:</label>
          <input type="email" v-model="email" class="input">
        </div>
        <div class="input-container">
          <label class="label">Message:</label>
          <textarea v-model="message" class="input"></textarea>
        </div>
        <button 
          @click="requestRemoval"
          :disabled="isSubmitDisabled"
          class="button mt-4"
        >
          Submit
        </button>
      </div>
      <div v-if="removalResponse.length > 0" class="removal-response mt-4">
        <h2>Removal Response</h2>
        <div v-for="response in removalResponse" :key="response.mask" class="response-item">
          <p><strong>Mask:</strong> {{ response.mask }}</p>
          <p><strong>Reason:</strong> {{ response.reason }}</p>
          <p><strong>Auto Removed:</strong> {{ response.autoremove ? 'Yes' : 'No' }}</p>
          <p><strong>Message:</strong> {{ response.msg }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
const formatDuration = (timestamp) => {
  const now = Math.floor(Date.now() / 1000)
  const diffSeconds = Math.abs(timestamp - now)
  
  const days = Math.floor(diffSeconds / (24 * 60 * 60))
  const hours = Math.floor((diffSeconds % (24 * 60 * 60)) / (60 * 60))
  const minutes = Math.floor((diffSeconds % (60 * 60)) / 60)
  
  const parts = []
  if (days > 0) parts.push(`${days} days`)
  if (hours > 0) parts.push(`${hours} hours`)
  if (minutes > 0) parts.push(`${minutes} minutes`)
  
  return parts.join(', ')
}

const formatDate = (timestamp) => {
  const date = new Date(timestamp * 1000)
  return new Intl.DateTimeFormat('default', {
    dateStyle: 'full',
    timeStyle: 'long',
  }).format(date)
}

const formatReason = (reason) => {
  return reason.replace(/\\u0026/g, '&')
}
</script>

<style scoped>
/* Add your styles here */
.container {
  max-width: 2xl;
  margin: auto;
  padding: 2rem 1rem;
}

.input-container {
  display: flex;
  gap: 1rem;
  align-items: center;
  margin-bottom: 1rem;
}

.label {
  white-space: nowrap;
}

.input {
  flex: 1;
  padding: 0.75rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.25rem;
}

.button {
  padding: 0.5rem 1rem;
  background-color: #4299e1;
  color: white;
  border-radius: 0.25rem;
  cursor: pointer;
}

.button:hover {
  background-color: #3182ce;
}

.error {
  color: #e53e3e;
}

.table-container {
  max-width: 2xl;
  margin: auto;
}

.table-auto {
  width: 100%;
  border-collapse: collapse;
}

.table-header {
  background-color: #4a5568;
  color: #ffffff;
  text-align: left;
  border: 1px solid #e2e8f0;
  padding: 0.5rem;
}

.table-cell {
  border: 1px solid #e2e8f0;
  padding: 0.5rem;
}

.table-auto tbody tr:nth-child(even) {
  background-color: #edf2f7;
}

.request-form .input-container {
  margin-bottom: 1rem;
}

.request-form .input {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.25rem;
}

.request-form .button:disabled {
  background-color: #a0aec0;
  cursor: not-allowed;
}

.removal-response {
  margin-top: 1rem;
}

.response-item {
  border: 1px solid #e2e8f0;
  padding: 1rem;
  border-radius: 0.25rem;
  margin-bottom: 1rem;
}
</style>

