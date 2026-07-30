import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './app/App'
import './i18n/config'
import 'sonner/dist/styles.css'
import './styles/globals.css'

const savedTheme = localStorage.getItem('imagesilo_theme')
document.documentElement.dataset.theme = savedTheme === 'light' || savedTheme === 'dark'
  ? savedTheme
  : (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')

const root = document.getElementById('root')
if (!root) {
  throw new Error('ImageSilo root element is missing')
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
