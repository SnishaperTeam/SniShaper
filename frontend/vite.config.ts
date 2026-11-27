import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'
import path from 'path'
import { wailsMockPlugin } from './mock/wails-mock'

export default defineConfig({
  // SNISHAPER_MOCK=1 npm run dev -> serve /wails/runtime from mock/wails-mock.ts,
  // so the UI can be previewed in a plain browser without the Go backend.
  plugins: [react(), ...(process.env.SNISHAPER_MOCK ? [wailsMockPlugin()] : [])],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'esnext',
    cssMinify: 'lightningcss',
    minify: 'oxc',
    reportCompressedSize: false,
    chunkSizeWarningLimit: 1000,
    rolldownOptions: {
      external: (id: string) => id.startsWith('/wails/'),
      onwarn(warning: any, warn: any) {
        if (warning.code === 'MODULE_LEVEL_DIRECTIVE' && warning.message.includes("'use client'")) return
        if (warning.code === 'CIRCULAR_DEPENDENCY') return
        warn(warning)
      },
    },
  },
  cacheDir: 'node_modules/.vite',
})
