import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'esnext',
    minify: 'oxc',
    reportCompressedSize: false,
    rolldownOptions: {
      external: (id: string) => id.startsWith('/wails/'),
      onwarn(warning: any, warn: any) {
        if (warning.code === 'MODULE_LEVEL_DIRECTIVE' && warning.message.includes("'use client'")) {
          return
        }
        if (warning.code === 'CIRCULAR_DEPENDENCY') {
          return
        }
        warn(warning)
      },
    },
  },
})
