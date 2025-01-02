import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '/api2': {
        target: 'http://localhost:2000',
        changeOrigin: true,
        //rewrite: (path) => path.replace(/^\/api/, ''), // Optional
      },
      '/api': {
        target: 'http://localhost:2001',
        changeOrigin: true,
        //rewrite: (path) => path.replace(/^\/api/, ''), // Optional
      },
    },
  },
})