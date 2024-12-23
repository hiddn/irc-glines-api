// src/views/ResultsPage.vue
<script setup>
import { computed } from 'vue'

const props = defineProps({
  glines: Array
})

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

<template>
  <div class="max-w-2xl mx-auto">
    <div v-if="glines.length === 0" class="text-center text-gray-500">
      No results found.
    </div>
    <div v-for="gline in glines" :key="gline.mask" class="mb-8 p-6 border rounded shadow">
      <h2 class="text-xl font-bold mb-4">G-line Information</h2>
      
      <div class="space-y-3">
        <p><strong>Mask:</strong> {{ gline.mask }}</p>
        
        <p>
          <strong>Status:</strong>
          {{ gline.expirets * 1000 > Date.now() 
            ? `Expires in ${formatDuration(gline.expirets)}`
            : `Expired ${formatDuration(gline.expirets)} ago`
          }}
        </p>
        
        <p><strong>Expiration Date:</strong> {{ formatDate(gline.expirets) }}</p>
        
        <p><strong>Reason:</strong> {{ formatReason(gline.reason) }}</p>
      </div>
    </div>
    
    <router-link 
      to="/"
      class="inline-block px-4 py-2 bg-gray-500 text-white rounded hover:bg-gray-600"
    >
      Back to Search
    </router-link>
  </div>
</template>

