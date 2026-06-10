import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN'
import enUS from './locales/en-US'

const savedLocale = localStorage.getItem('mistdocs-locale') || 'zh-CN'

const i18n = createI18n({
  legacy: false,
  locale: savedLocale,
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

export default i18n

export function setLocale(locale: string) {
  ;(i18n.global as any).locale.value = locale
  localStorage.setItem('mistdocs-locale', locale)
  document.documentElement.setAttribute('lang', locale === 'zh-CN' ? 'zh-CN' : 'en')
}

export function getLocale(): string {
  return (i18n.global as any).locale.value
}
