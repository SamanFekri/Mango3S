/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// CodeMirror breaks ("Unrecognized extension value… multiple instances of
// @codemirror/state") if its core packages get bundled more than once, e.g.
// when a newly added language package is pre-bundled separately from the
// rest. One copy each, and pre-bundled together.
const codemirror = [
  'codemirror',
  '@codemirror/state',
  '@codemirror/view',
  '@codemirror/language',
  '@codemirror/autocomplete',
  '@codemirror/commands',
  '@codemirror/lang-javascript',
  '@codemirror/lang-sql',
  '@codemirror/lang-python',
  '@codemirror/lang-java',
  '@codemirror/lang-go',
  '@codemirror/legacy-modes/mode/clike',
  '@lezer/common',
  '@lezer/highlight',
  '@lezer/lr',
]

export default defineConfig({
  plugins: [react()],
  resolve: {
    dedupe: ['@codemirror/state', '@codemirror/view', '@codemirror/language', '@lezer/common', '@lezer/highlight', '@lezer/lr'],
  },
  optimizeDeps: {
    include: codemirror,
  },
  build: {
    chunkSizeWarningLimit: 1500,
  },
  // The About dialog imports ../LICENSE (the repository's license text).
  server: {
    fs: { allow: ['..'] },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
