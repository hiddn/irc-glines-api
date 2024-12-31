<script setup>
import { ref, computed, onMounted } from 'vue'
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
const paramIP = ref('')

const showRequestForm = ref(false)
const requestButtonEnabled = ref(false)
const nickname = ref('')
const realname = ref('')
const email = ref('')
const user_message = ref('')
const uuid = ref('')
const emailConfirmed = ref('')
const gotGlinesResults = ref(false)

const timerTasksId = ref(null)

const isSubmitEnabled = ref(true)
const isAllFieldsNonEmpty = computed(() => {
  return !nickname.value || !realname.value || !email.value || !user_message.value
})

onMounted(() => {
  console.debug("onMounted")
  const input_ip = document.getElementById('input_ip');
  const params = new URLSearchParams(window.location.search);
  paramIP.value = params.get('ip')
  if (paramIP.value != null) {
      ip.value = paramIP.value
      input_ip.value = paramIP.value;
      input_ip.focus();
      input_ip.select();
    }
  getUserIP()
})

function startTasks() {
  if (timerTasksId.value === null) {
    timerTasksId.value = setInterval(GetTasks, 5000)
  }
}

function stopTasks() {
  if (timerTasksId.value !== null) {
    clearInterval(timerTasksId.value)
    timerTasksId.value = null
  }
}

const removalResponse = ref([])

// Try to get user's IP address
const getUserIP = async () => {
  try {
    const response = await axios.get('https://api.ipify.org?format=json')
    myip.value = response.data.ip
    if (paramIP.value == null) {
      const input_ip = document.getElementById('input_ip');
      ip.value = response.data.ip
      input_ip.value = response.data.ip;
      input_ip.focus();
      input_ip.select();
    }
  } catch (error) {
    console.error('Failed to get user IP:', error)
  }
}

const lookupGline = async () => {
  if (!ip.value) return
  
  errormsg.value = ''
  removalResponse.value = ''
  glines.value = []
  isSubmitEnabled.value = true
  showRequestForm.value = false
  gotGlinesResults.value = false
  requestButtonEnabled.value = true
  const url = config.glinelookup_url
    .replace(':network', config.network.toLowerCase())
    .replace(':ip', ip.value)
    
  try {
    const response = await axios.get(url, {
      headers: {
        'Content-Type': 'application/json',
        //'Authorization': `Bearer ${config.api_key}`
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
    uuid: uuid.value,
    network: config.network,
    ip: ip.value,
    nickname: nickname.value,
    realname: realname.value,
    email: email.value,
    user_message: user_message.value
  }
  isSubmitEnabled.value = true
  requestButtonEnabled.value = false

  try {
    const response = await axios.post('/api/requestrem', requestData, {
      headers: {
        'Content-Type': 'application/json',
        //'Authorization': `Bearer ${config.api_key}`
      }
    })
    uuid.value = response.data.uuid
    if (response.status === 202) {
      errormsg.value = response.data.message
      showRequestForm.value = false
      //isSubmitEnabled.value = true
      startTasks()
    }
    else if (response.status === 200) {
      //removalResponse.value = response.data.glines
      errormsg.value = ''
      glines.value = response.data.glines
      gotGlinesResults.value = true
      isSubmitEnabled.value = false
      showRequestForm.value = false
      if (response.data.request_sent_via_email == true) {
        errormsg.value = 'Removal request sent via email to the Abuse Team.'
      }
    }
    console.log('Removal request response:', response.data)
  } catch (error) {
    requestButtonEnabled.value = true
    console.error('Failed to request removal:', error)
    if (error.status === 400) {
      errormsg.value = 'Invalid request data: ' + error.response.data
      return
    }
    errormsg.value = 'API call fail for requestRemoval(): ' + error.status + error.response.data
  }
}

const GetTasks = async () => {
  try {
    const response = await axios.get(`/api/tasks/${uuid.value}`, {
      headers: {
        'Content-Type': 'application/json'
      }
    })
    const tasks = response.data
    tasks.forEach(task => {
      if (task.task_type === 'confirmemail') {
        if (task.progress === 100) {
          emailConfirmed.value = task.data
          console.log('Email confirmed:', emailConfirmed.value)
          errormsg.value = 'Email confirmed. Submitting removal request...'
          requestRemoval()
          stopTasks()
        }
      }
    })
  } catch (error) {
    console.error('Failed to get tasks:', error)
  }
}

const handleKeyPress = (event) => {
  if (event.key === 'Enter') {
    lookupGline()
  }
}

</script>

<template>
  <div class="container mx-auto px-4 py-8">
    <h1>Gline Lookup</h1>
    <p>Your IP: {{ myip }}</p>
    <div class="input-container">
      <label class="label">IP address:</label>
      <input 
        id="input_ip"
        type="text"
        v-model="ip"
        class="input"
        autofocus
        onfocus="this.select();"
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
      <span class="label-title">Search results</span>
      <table class="table-auto">
        <thead>
          <tr>
            <th class="table-header">Mask</th>
            <th class="table-header">Reason</th>
            <th class="table-header">Expiration</th>
            <th v-if="gotGlinesResults" class="table-header">Request status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="gline in glines" :key="gline.mask">
            <td class="table-cell">{{ gline.mask }}</td>
            <td class="table-cell">{{ formatReason(gline.reason) }}</td>
            <td class="table-cell">
              <span v-html="getExpirationString(gline)"></span>
            </td>
            <td v-if="gotGlinesResults" class="table-cell gline-results">{{ gline.message }}</td>
          </tr>
        </tbody>
      </table>
      <button 
        v-if="!showRequestForm && requestButtonEnabled"
        @click="showRequestForm = true"
        class="button mt-4"
      >
        Request removal
      </button>
    </div>
      <div v-if="showRequestForm" class="request-form mt-4">
        <div class="form-row">
          <label class="label">Nickname:</label>
          <input type="text" v-model="nickname" class="input">
        </div>
        <div class="form-row">
          <label class="label">Real Name:</label>
          <input type="text" v-model="realname" class="input">
        </div>
        <div class="form-row">
          <label class="label">Email:</label>
          <input type="email" v-model="email" class="input">
        </div>
        <div class="form-row">
          <label class="label">Message:</label>
          <textarea v-model="user_message" class="input"></textarea>
        </div>
        <div class="form-button">
          <button 
            id="removeButton"
            @click="requestRemoval"
            :disabled="isAllFieldsNonEmpty || !isSubmitEnabled"
            class="button mt-4"
          >
            Submit
          </button>
        </div>
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

const getExpirationString = (gline) => {
  /*
  <span v-if="!gline.active" style="color: red;">Deactivated</span>
              <span v-if="gline.active">
                {{ formatDate(gline.expirets) }}
                <span v-html="'<br/>'"></span>
                <span v-if="gline.active && (gline.expirets * 1000) <= Date.now()" style="color: red;" v-html="'EXPIRED'"></span>
                {{ gline.expirets * 1000 > Date.now()
                    ? `-> in ${formatDuration(gline.expirets)})`
                    : ` ${formatDuration(gline.expirets)} ago` }}
              </span>
  */
  let exp = ''
  let isExpired = (gline.expirets * 1000) <= Date.now()
  if (!gline.active) {
    exp = '<span style="color: red;">Deactivated</span>'
    return exp
  }
  exp = `${formatDate(gline.expirets)}<br/>`
  if (isExpired) {
    exp += '<b><span style="color: green;">EXPIRED</span>: '
  }
  else {
    exp += '(<b>in '
  }
  exp += `${formatDuration(gline.expirets)}`
  if (isExpired) {
    exp += '</b> ago'
  }
  else {
    exp += '</b>)'
  }
  return exp
}

const formatReason = (reason) => {
  return reason.replace(/\\u0026/g, '&')
}
</script>

<style>
/* Add your styles here */
body {
  display: block;
}
#app {
  width: 100%;
}
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
  margin-bottom: 2rem;
}

.table-auto {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 2rem;
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
  text-align: left;
}

.table-auto tbody tr:nth-child(even) {
  background-color: black;
}

/* Container for the form */
.request-form {
  display: flex;
  flex-direction: column; /* Stack rows vertically */
  gap: 1rem; /* Space between rows */
}

/* Label and input row styling */
.form-row {
  display: flex;
  align-items: left; /* Align label and input vertically */
  gap: 1rem; /* Space between label and input */
}

/* Label styling */
.request-form label {
  width: 16rem; /* Fixed width for labels */
  text-align: left; /* Align label text to the right */
  font-size: 1rem;
  font-weight: bold;
}

/* Input styling */
.request-form textarea,
.request-form input[type="text"],
.request-form input[type="email"],
.request-form input[type="password"] {
  flex: 1; /* Allow inputs to grow to fill remaining space */
  padding: 0.5rem;
  font-size: 1rem;
  border: 1px solid #ccc;
  border-radius: 5px;
}

.request-form button {
  max-width: 10rem;
}
.form-button {
  align-items: right;
}


.request-form .input-container {
  margin-bottom: 1rem;
}

.input-container label {
  font-weight: bold;
}

.label-title {
  display: block;
  text-align: left;
  margin-bottom: 1rem;
  margin-top: 3rem;
  font-weight: bold;
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
.gline-results {
  color: black;
  background-color: yellow;
}
.colorred {
  color: red;
}
</style>

