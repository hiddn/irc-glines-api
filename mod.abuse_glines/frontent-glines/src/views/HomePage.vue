// src/views/HomePage.vue
<script setup>
import { ref } from 'vue'

const ip = ref('')
const error = ref('')

const handleKeyPress = (event) => {
  if (event.key === 'Enter') {
    // Emit lookup event to parent component
    emit('lookup')
  }
}

const emit = defineEmits(['lookup', 'keypress'])
</script>

<template>
  <div class="max-w-xl mx-auto">
    <h1>Gline Lookup</h1>
    <p>Your IP: {{ ip }}</p>
    <div class="flex gap-4 items-center mb-4">
      <label class="whitespace-nowrap">IP address:</label>
      <input 
        type="text"
        v-model="ip"
        class="flex-1 px-3 py-2 border rounded"
        @keypress="handleKeyPress"
      >
      <button 
        @click="emit('lookup')"
        class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
      >
        Lookup Gline
      </button>
    </div>
    
    <p v-if="error" class="text-red-600">{{ error }}</p>
  </div>
</template>

<style scoped>
/* Add your styles here */
</style>

