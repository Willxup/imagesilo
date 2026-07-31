import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import enUS from './en-US.json'
import zhCN from './zh-CN.json'
import { readLocalStorage } from '../lib/browser-storage'

function updateDocumentLanguage(language: string) {
  if (typeof document !== 'undefined') document.documentElement.lang = language === 'zh-CN' ? 'zh-CN' : 'en'
}

const initialLanguage = readLocalStorage('imagesilo_language') === 'en-US' ? 'en-US' : 'zh-CN'

void i18n
  .use(initReactI18next)
  .init({
    resources: {
      'zh-CN': { translation: zhCN },
      'en-US': { translation: enUS },
    },
    lng: initialLanguage,
    fallbackLng: 'en-US',
    interpolation: { escapeValue: false },
  })
  .then(() => updateDocumentLanguage(i18n.resolvedLanguage ?? i18n.language))

i18n.on('languageChanged', updateDocumentLanguage)

export default i18n
