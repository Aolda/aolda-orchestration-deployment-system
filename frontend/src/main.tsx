import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createTheme, MantineProvider } from '@mantine/core'
import '@mantine/core/styles.css'
import './index.css'
import App from './App.tsx'

const theme = createTheme({
  primaryColor: 'lagoon',
  colors: {
    lagoon: [
      '#ecf7f5',
      '#d3eeea',
      '#a8ddd6',
      '#7ccbc1',
      '#58bfb4',
      '#41b4a8',
      '#309084',
      '#236e65',
      '#184f49',
      '#0f3330',
    ],
    sand: [
      '#fbf5ea',
      '#f4e8c7',
      '#ecd89a',
      '#e3c769',
      '#dbbb45',
      '#d5b129',
      '#bb9818',
      '#947710',
      '#69540a',
      '#403304',
    ],
    coral: [
      '#fff0ea',
      '#ffd8c9',
      '#ffb49c',
      '#ff8c6b',
      '#f96f48',
      '#f55e32',
      '#d9471d',
      '#ab3513',
      '#78240c',
      '#481305',
    ],
  },
  fontFamily: '"Avenir Next", "Segoe UI", sans-serif',
  headings: {
    fontFamily: '"Iowan Old Style", "Palatino Linotype", serif',
  },
  defaultRadius: 'md',
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="light">
      <App />
    </MantineProvider>
  </StrictMode>,
)
