import type { UserConfig } from 'vite'
import react from '@vitejs/plugin-react'

type VitestConfig = {
  test: {
    environment: 'jsdom'
    setupFiles: string
    css: boolean
    include: string[]
    restoreMocks: boolean
  }
}

const config = {
  plugins: [react()],
  server: {
    host: 'localhost',
    port: 5173,
    strictPort: true,
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/testing/setup.ts',
    css: true,
    include: ['src/**/*.test.{ts,tsx}'],
    restoreMocks: true,
  },
} satisfies UserConfig & VitestConfig

export default config
