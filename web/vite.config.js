import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      // Dev convenience: keep frontend code using same-origin paths.
      // Backend default: http://localhost:19527
      '/api': {
        target: 'http://localhost:19527',
        changeOrigin: true,
      },
      '/analysis': {
        target: 'http://localhost:19527',
        changeOrigin: true,
      },
    },
  },
})
