import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { tanstackRouter } from '@tanstack/router-plugin/vite'

export default defineConfig({
  publicDir: false,
  plugins: [
    tanstackRouter({
      routeFileIgnorePattern: '^(workbench|editor-|art\\.|ios-|reader-auth|api\\.|robots|sitemap|healthz)',
      generatedRouteTree: './src/routeTree.public.gen.js',
      disableTypes: true,
      autoCodeSplitting: true,
    }),
    react(),
  ],
  build: { outDir: 'dist/site' },
})
