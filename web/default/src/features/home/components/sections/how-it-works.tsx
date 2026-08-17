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
import { Database, FileCheck2, Gauge, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

export function HowItWorks() {
  const { t } = useTranslation()
  const advantages = [
    {
      icon: FileCheck2,
      title: t('home.advantages.invoiceTitle'),
      description: t('home.advantages.invoiceDescription'),
    },
    {
      icon: Database,
      title: t('home.advantages.poolTitle'),
      description: t('home.advantages.poolDescription'),
    },
    {
      icon: Gauge,
      title: t('home.advantages.speedTitle'),
      description: t('home.advantages.speedDescription'),
    },
    {
      icon: ShieldCheck,
      title: t('home.advantages.stableTitle'),
      description: t('home.advantages.stableDescription'),
    },
  ]

  return (
    <section className='border-border/50 bg-muted/20 relative z-10 border-y px-4 py-20 sm:px-6 md:py-24'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='max-w-2xl'>
          <p className='mb-3 text-xs font-semibold text-orange-600 dark:text-orange-400'>
            CORE ADVANTAGES
          </p>
          <h2 className='text-3xl leading-tight font-bold md:text-4xl'>
            {t('home.advantages.title')}
          </h2>
          <p className='text-muted-foreground mt-4 text-sm leading-6 md:text-base'>
            {t('home.advantages.description')}
          </p>
        </AnimateInView>

        <div className='border-border/60 mt-12 grid gap-0 border-y md:grid-cols-4'>
          {advantages.map((advantage, index) => {
            const Icon = advantage.icon
            return (
              <AnimateInView
                key={advantage.title}
                delay={index * 90}
                animation='fade-up'
                className={`py-7 md:px-6 ${
                  index > 0
                    ? 'border-border/60 border-t md:border-t-0 md:border-l'
                    : ''
                }`}
              >
                <Icon
                  className='size-6 text-emerald-600 dark:text-emerald-400'
                  strokeWidth={1.7}
                  aria-hidden='true'
                />
                <h3 className='mt-4 text-base font-semibold'>
                  {advantage.title}
                </h3>
                <p className='text-muted-foreground mt-2 text-sm leading-6'>
                  {advantage.description}
                </p>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
