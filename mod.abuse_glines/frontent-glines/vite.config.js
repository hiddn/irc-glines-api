import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '/glinelookup': {
        target: 'http://localhost:2000',
        changeOrigin: true,
        //rewrite: (path) => path.replace(/^\/api/, ''), // Optional
      },
    },
  },
})