<script setup>
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import './style.css'

const config = {
  network: 'Undernet',
  glinelookup_url: '/api2/glinelookup/:network/:ip',
  recaptcha_site_key: '6LfHTqwqAAAAANeeZSuospbpzNBPbfQhMnAo2rwu'
}

const myip = ref('')
const ip = ref('')
const errormsg = ref('')
const glines = ref([])
const paramIP = ref('')
const recaptchaToken = ref('')
const recaptchaModalVisible = ref(false)

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
  //loadRecaptcha()
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
      errormsg.value = 'No G-lines found'
    }

  } catch (error) {
    if (error.response && error.response.status === 400) {
      errormsg.value = 'Invalid IP address'
      return
    }
    errormsg.value = 'API call error ' + error.response.status + ': ' + error.response.data
    //errormsg.value = 'Failed to lookup G-line: ' + error.message
    console.error('Failed to lookup G-line:', error)
  }
}

const requestRemoval = async (needRecaptcha) => {
  if (needRecaptcha && !recaptchaToken.value) {
    loadRecaptcha()
    return
  }
  recaptchaModalVisible.value = false
  const requestData = {
    uuid: uuid.value,
    network: config.network,
    ip: ip.value,
    nickname: nickname.value,
    realname: realname.value,
    email: email.value,
    user_message: user_message.value,
    'recaptcha_token': recaptchaToken.value
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
    if (error.status === 403) {
      loadRecaptcha()
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
          requestRemoval(false)
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

const loadRecaptcha = () => {
  recaptchaToken.value = ''
  recaptchaModalVisible.value = true
  if (window.grecaptcha) {
    window.grecaptcha.render('recaptcha', {
      sitekey: config.recaptcha_site_key,
      callback: recaptchaCB,
      "expired-callback": reloadRecaptcha,
      "error-callback": reloadRecaptcha,
      theme: "dark",
      size: "compact",
    })
  }
  else {
    alert("Recaptcha not found")
  }
}
const reloadRecaptcha = () => {
  window.grecaptcha.reset();
}
const recaptchaCB = async () => {
  const recaptchaResponse = window.grecaptcha.getResponse()
  if (!recaptchaResponse) {
    alert('Please complete the reCAPTCHA.')
    return
  }

  recaptchaToken.value = recaptchaResponse
  recaptchaModalVisible.value = false
  return

  // Submit the reCAPTCHA token to the backend
  const response = await fetch('/api/verify-captcha', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: recaptchaResponse }),
  })
  const result = await response.json()
  if (response.status != 200) {
    alert('Verification failed.')
    reloadRecaptcha()
  }
}

function toggleShowRequestForm() {
  showRequestForm.value = !showRequestForm.value
}

</script>

<template>
  <div class="container mx-auto px-4 py-8">
    <h1>G-line Lookup</h1>
    <p>Your IP: {{ myip }}</p>
    <div class="input-container">
      <label for="input_ip" class="label">IP address:</label>
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
        Lookup
      </button>
    </div>
    
    <p v-if="errormsg" class="error">{{ errormsg }}</p>
    
    <div v-if="glines.length > 0" class="table-container">
      <span class="label-title">G-lines:</span>
      <div v-for="gline in glines" :key="gline.mask" class="gline">
        <div>
          <span class="gline-info-title"></span>
          <span class="gline-mask">{{ gline.mask }}</span>
        </div>
        <dl class="key-value-list gline-infos">
          <div>
            <dt>Reason:</dt>
            <dd>{{ formatReason(gline.reason) }}</dd>
          </div>
          <div>
            <dt>Expiration: </dt>
            <dd v-html="getExpirationString(gline)"></dd>
          </div>
        </dl>
        <div v-if="gotGlinesResults" class="gline-results">
          {{ gline.message }}
        </div>
      </div>
      <button 
        v-if="!showRequestForm"
        @click="toggleShowRequestForm()"
        class="button mt-4"
      >
        Request removal
      </button>
    </div>
      <div v-if="showRequestForm" class="request-form mt-4 key-value-list">
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
            @click="requestRemoval(true)"
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
  <div id="recaptcha-modal" tabindex="1" class="modal" v-bind:style="{ display: recaptchaModalVisible ? 'block' : 'none' }">
    <div id="recaptcha" class="g-recaptcha" :data-sitekey="config.recaptcha_site_key"></div>
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
    exp = '<span style="color: green;">Deactivated</span>'
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
body {
  max-width: 1100px;
  align-items: flex-start;
  margin: auto;
  display: flex;
}
#app {
  width: 100%;
  padding: 0rem 2rem;
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
  grid-column: span 2;
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
  font-size: 1.25rem
}

.request-form .input {
  padding: 0.75rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.25rem;
}

.request-form .button:disabled {
  background-color: #a0aec0;
  cursor: not-allowed;
  color: black;
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
  padding: 0.5rem;
  border-radius: 0.25rem;
}
.colorred {
  color: red;
}
.gline {
  background-color: rgb(29, 28, 28);
  text-align: left;
  margin: auto;
  padding: 0.5rem;
  margin: 1rem 0rem;
  border-radius: 0.5rem;
}
.gline-info-title {
  font-weight: bold;
  font-size: 1.25rem;
}
.gline-mask {
  margin-left: 0.5rem;
  color: lightseagreen;
  font-size: 1.25rem;
}
.key-value-list {
  display: grid;
  grid-template-columns: auto 1fr; /* Keys in the first column, values in the second column */
  gap: 1rem 2rem; /* Spacing between rows and columns */
  align-items: center; /* Align keys and values vertically */
}
.gline-infos {
  padding-left: 2rem;
}
.key-value-list > div {
  display: contents; /* Ensures each pair behaves as a row */
}
.key-value-list dt {
  font-weight: bold;
  margin: 0;
  align-self: start;
}
.key-value-list dd {
  margin: 0;
  align-self: start;
}

recaptcha-modal {
  display: none;
}

/* The Modal (background) */
.modal {
  display: none; /* Hidden by default */
  position: fixed; /* Stay in place */
  z-index: 1; /* Sit on top */
  padding-top: 20px; /* Location of the box */
  left: 0;
  top: 0;
  width: 100%; /* Full width */
  height: 100%; /* Full height */
  min-height: 200px;
  overflow: auto; /* Enable scroll if needed */
  background-color: rgb(0,0,0); /* Fallback color */
  background-color: rgba(0,0,0,0.9); /* Black w/ opacity */
  text-align: center;
  align-content: center;
}

.g-recaptcha {
  display: grid;
  justify-content: center;
}


@media (max-width: 768px) {
  /* Mobile styles here */
  body {
    font-size: 1rem;
    margin: 0rem;
    padding: 0rem;
  }
  #app {
    padding: 0rem 0rem;
  }
  .input-container {
    display: grid;
    align-items: start;
    margin: auto;
  }
  .gline-infos {
    padding-left: 0.5rem;
  }

  .key-value-list {
    font-size: 0.8rem;
    gap: 1rem 0.5rem;
  }
  button {
    width: 100%;
  }
  .label {
    align-self: start;
    text-align: left;
  }
  h1 {
    font-size: 2.5rem;
  }
  .input {
    flex: 1;
    padding: 0.75rem;
    border: 1px solid #e2e8f0;
    border-radius: 0.25rem;
  }
}
</style>

