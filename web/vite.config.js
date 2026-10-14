import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    // Listen on all addresses: IPv6 (::) when available, which on most systems
    // also accepts IPv4, otherwise IPv4 (0.0.0.0). By default Vite may bind
    // only to ::1, and then http://localhost:5173 does not open in some browsers.
    host: true,
    // In development the Go backend runs separately on :8080.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
