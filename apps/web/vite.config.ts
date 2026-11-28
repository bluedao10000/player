import { fileURLToPath } from 'node:url'
import { serverKnowledgeApiUrl } from '@anime/shared/knowledge'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'

export default defineConfig({
  resolve: {
    alias: { '../lib/document': fileURLToPath(new URL('./src/lib/document.local.tsx', import.meta.url)) },
  },
  plugins: [tanstackStart(), react()],
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WEB_PORT ?? 4310),
    strictPort: true,
    proxy: { '/api/v1/knowledge': serverKnowledgeApiUrl() },
  },
})
