import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './app/App'
import './i18n/config'
import './styles/globals.css'

const root = document.getElementById('root')
if (!root) {
  throw new Error('ImageSilo root element is missing')
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
