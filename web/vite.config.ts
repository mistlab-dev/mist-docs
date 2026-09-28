import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// Dev proxy target; override to run several local stacks side by side,
// e.g. VITE_API_TARGET=http://127.0.0.1:8920 npx vite --port 3020
const apiTarget = process.env.VITE_API_TARGET || 'http://localhost:8900'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
      },
      '/ws': {
        target: apiTarget.replace(/^http/, 'ws'),
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      output: {
        manualChunks: {
          'element-plus': ['element-plus', '@element-plus/icons-vue'],
          vue: ['vue', 'vue-router', 'pinia'],
          yjs: ['yjs', '@tiptap/extension-collaboration'],
        },
      },
    },
  },
})
