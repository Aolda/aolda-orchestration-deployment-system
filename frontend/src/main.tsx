import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createTheme, MantineProvider } from '@mantine/core'
import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'
import { Notifications } from '@mantine/notifications'
import './index.css'
import App from './App.tsx'

const theme = createTheme({
  primaryColor: 'lagoon',
  colors: {
    lagoon: [
      '#eef5ff',
      '#d8e8ff',
      '#b3d0ff',
      '#85b6ff',
      '#5f9bfb',
      '#3f83f4',
      '#1d66d6',
      '#1352ab',
      '#0b3d7f',
      '#042754',
    ],
  },
  fontFamily:
    '"Pretendard Variable", "Pretendard", -apple-system, BlinkMacSystemFont, system-ui, Roboto, "Helvetica Neue", "Segoe UI", "Apple SD Gothic Neo", "Noto Sans KR", "Malgun Gothic", "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", sans-serif',
  headings: {
    fontFamily:
      '"Pretendard Variable", "Pretendard", -apple-system, BlinkMacSystemFont, system-ui, Roboto, "Helvetica Neue", "Segoe UI", "Apple SD Gothic Neo", "Noto Sans KR", "Malgun Gothic", "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", sans-serif',
  },
  defaultRadius: 'md',
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="light">
      <Notifications position="top-right" zIndex={2000} />
      <App />
    </MantineProvider>
  </StrictMode>,
)
