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
import { Building2, Database, Network } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

export function HowItWorks() {
  const { t } = useTranslation()
  return (
    <section className='border-b border-white/[0.08] bg-[#0a101a] px-4 py-14 sm:px-6 md:py-16'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView className='max-w-3xl'>
          <p className='mb-3 text-xs font-semibold text-emerald-300'>
            RELIABLE BY DESIGN
          </p>
          <h2 className='text-2xl leading-tight font-bold text-white md:text-3xl'>
            {t('home.capabilities.title')}
          </h2>
          <p className='mt-4 text-sm leading-6 text-white/48 md:text-base'>
            {t('home.capabilities.description')}
          </p>
        </AnimateInView>

        <AnimateInView
          delay={80}
          className='mt-8 grid items-stretch border-y border-white/[0.09] md:grid-cols-[1fr_auto_1fr_auto_1fr]'
        >
          {[
            {
              icon: Network,
              label: t('home.flow.unified'),
              detail: 'OpenAI / Anthropic / Gemini',
            },
            {
              icon: Database,
              label: t('home.flow.pool'),
              detail: t('home.flow.scheduling'),
            },
            {
              icon: Building2,
              label: t('home.flow.delivery'),
              detail: t('home.flow.enterprise'),
            },
          ].map((step, index) => {
            const Icon = step.icon
            return (
              <div key={step.label} className='contents'>
                <div className='flex items-center gap-4 px-2 py-5 md:px-6'>
                  <span className='flex size-10 shrink-0 items-center justify-center rounded-lg border border-blue-300/15 bg-blue-300/[0.06] text-cyan-300'>
                    <Icon className='size-5' />
                  </span>
                  <span>
                    <strong className='block text-sm text-white'>
                      {step.label}
                    </strong>
                    <span className='mt-1 block text-[10px] text-white/38'>
                      {step.detail}
                    </span>
                  </span>
                </div>
                {index < 2 ? (
                  <div className='hidden items-center text-blue-300/35 md:flex'>
                    <span className='h-px w-8 bg-current' />
                    <span className='size-1.5 rotate-45 border-t border-r border-current' />
                  </div>
                ) : null}
              </div>
            )
          })}
        </AnimateInView>
      </div>
    </section>
  )
}
