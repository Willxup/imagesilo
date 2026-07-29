import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import enUS from './en-US.json'
import zhCN from './zh-CN.json'

void i18n.use(initReactI18next).init({
  resources: {
    'zh-CN': { translation: zhCN },
    'en-US': { translation: enUS },
  },
  lng: globalThis.localStorage?.getItem('imagesilo_language') === 'en-US' ? 'en-US' : 'zh-CN',
  fallbackLng: 'en-US',
  interpolation: { escapeValue: false },
})

export default i18n
