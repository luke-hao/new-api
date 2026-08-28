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
import { lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { CTA, Features, Hero, HowItWorks, Stats } from './components'
import { useHomeModelCatalog, useHomePageContent } from './hooks'

const LazyMarkdown = lazy(() =>
  import('@/components/ui/markdown').then(({ Markdown }) => ({
    default: Markdown,
  }))
)

export function Home() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isUrl } = useHomePageContent()
  const modelSummary = useHomeModelCatalog()

  if (content) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='overflow-x-hidden'>
          {isUrl ? (
            <iframe
              src={content}
              className='h-screen w-full border-none'
              title={t('Custom Home Page')}
            />
          ) : (
            <div className='container mx-auto py-8'>
              <Suspense
                fallback={
                  <div className='bg-muted/40 min-h-40 animate-pulse' />
                }
              >
                <LazyMarkdown className='custom-home-content'>
                  {content}
                </LazyMarkdown>
              </Suspense>
            </div>
          )}
        </main>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout
      showMainContainer={false}
      showThemeSwitch={false}
      showNotifications={false}
      headerProps={{ appearance: 'dark' }}
    >
      <main className='dark bg-[#070b12] text-white'>
        <Hero isAuthenticated={isAuthenticated} modelSummary={modelSummary} />
        <Stats modelSummary={modelSummary} />
        <Features modelSummary={modelSummary} />
        <HowItWorks />
        <CTA isAuthenticated={isAuthenticated} />
        <Footer />
      </main>
    </PublicLayout>
  )
}
