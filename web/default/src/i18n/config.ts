/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import i18n, {
  type BackendModule,
  type ReadCallback,
  type ResourceKey,
} from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

interface LocaleBundle {
  translation: ResourceKey
}

const localeLoaders: Record<string, () => Promise<LocaleBundle>> = {
  en: () =>
    import('./locales/en.json').then(
      ({ default: locale }) => locale as LocaleBundle
    ),
  zh: () =>
    import('./locales/zh.json').then(
      ({ default: locale }) => locale as LocaleBundle
    ),
  fr: () =>
    import('./locales/fr.json').then(
      ({ default: locale }) => locale as LocaleBundle
    ),
  ru: () =>
    import('./locales/ru.json').then(
      ({ default: locale }) => locale as LocaleBundle
    ),
  ja: () =>
    import('./locales/ja.json').then(
      ({ default: locale }) => locale as LocaleBundle
    ),
  vi: () =>
    import('./locales/vi.json').then(
      ({ default: locale }) => locale as LocaleBundle
    ),
}

const localeBackend: BackendModule = {
  type: 'backend',
  init: () => undefined,
  read: (language: string, _namespace: string, callback: ReadCallback) => {
    const normalizedLanguage = language.split('-')[0]?.toLowerCase() || 'en'
    const loadLocale = localeLoaders[normalizedLanguage] ?? localeLoaders.en

    loadLocale()
      .then(({ translation }) => callback(null, translation))
      .catch((error: unknown) => {
        callback(
          error instanceof Error ? error : new Error('Failed to load locale'),
          null
        )
      })
  },
}

function applyDocumentLanguage(language: string) {
  if (typeof document === 'undefined') return
  document.documentElement.lang = language.split('-')[0]?.toLowerCase() || 'en'
}

i18n.on('languageChanged', applyDocumentLanguage)

i18n
  .use(LanguageDetector)
  .use(localeBackend)
  .use(initReactI18next)
  .init({
    fallbackLng: 'en',
    supportedLngs: ['en', 'zh', 'fr', 'ru', 'ja', 'vi'],
    load: 'languageOnly', // Convert zh-CN -> zh
    ns: ['translation'],
    defaultNS: 'translation',
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
    },
  })

export default i18n
