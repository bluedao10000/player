import { defineConfig } from 'vite'

export default defineConfig({
  build: { ssr: 'src/reader-api.ts', outDir: 'dist/reader-api' },
  ssr: { noExternal: ['@anime/shared'] },
})
