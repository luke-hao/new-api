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
import { Link } from '@tanstack/react-router'
import { ArrowUpRight, Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'
import type { HomeModelFamily, HomeModelSummary } from '../../types'
import { LazyLobeIcon } from '../lazy-lobe-icon'

const TONE_CLASSES: Record<HomeModelFamily['tone'], string> = {
  blue: 'border-blue-400/35 bg-blue-400/[0.045]',
  amber: 'border-amber-400/35 bg-amber-400/[0.045]',
  cyan: 'border-cyan-400/35 bg-cyan-400/[0.045]',
  slate: 'border-slate-300/25 bg-slate-300/[0.035]',
  rose: 'border-rose-400/35 bg-rose-400/[0.045]',
  mint: 'border-emerald-400/35 bg-emerald-400/[0.045]',
}

type FeaturesProps = {
  modelSummary: HomeModelSummary
}

export function Features({ modelSummary }: FeaturesProps) {
  const { t } = useTranslation()

  return (
    <section className='border-b border-white/[0.08] bg-[#070b12] px-4 py-20 sm:px-6 md:py-24'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView className='flex flex-col justify-between gap-5 md:flex-row md:items-end'>
          <div className='max-w-3xl'>
            <p className='mb-3 text-xs font-semibold text-cyan-300'>
              MODEL NETWORK / {modelSummary.displayTotal}
            </p>
            <h2 className='text-3xl leading-tight font-bold text-white md:text-4xl'>
              {t('home.catalog.title')}
            </h2>
          </div>
          <div className='max-w-md md:text-right'>
            <p className='text-sm leading-6 text-white/48'>
              {t('home.catalog.description')}
            </p>
            <Link
              to='/pricing'
              className='mt-3 inline-flex items-center gap-1 text-sm font-semibold text-blue-300 hover:text-cyan-200'
            >
              {t('home.catalog.action')}
              <ArrowUpRight className='size-4' />
            </Link>
          </div>
        </AnimateInView>

        <div className='mt-12 grid gap-3 md:grid-cols-2 lg:grid-cols-3'>
          {modelSummary.families.map((family, index) => (
            <AnimateInView
              key={family.id}
              delay={index * 70}
              className={`min-h-64 rounded-lg border border-t-2 p-5 ${TONE_CLASSES[family.tone]}`}
            >
              <div className='flex items-start justify-between gap-3'>
                <span className='flex size-10 items-center justify-center rounded-lg border border-white/10 bg-white/[0.055]'>
                  <LazyLobeIcon iconName={family.iconName} size={22} />
                </span>
                <span className='rounded-full border border-emerald-300/15 bg-emerald-300/[0.06] px-2 py-1 text-[9px] font-semibold text-emerald-200'>
                  {t('home.catalog.available')}
                </span>
              </div>
              <h3 className='mt-5 text-xl font-bold text-white'>
                {family.title}
              </h3>
              <p className='mt-2 min-h-10 text-xs leading-5 text-white/42'>
                {t(family.descriptionKey)}
              </p>
              <div className='mt-5 flex flex-wrap gap-1.5'>
                {family.models.map((model) => (
                  <span
                    key={model}
                    className='inline-flex max-w-full items-center gap-1.5 rounded-md border border-white/[0.07] bg-white/[0.035] px-2 py-1.5 font-mono text-[10px] text-white/65'
                  >
                    <Check className='size-3 shrink-0 text-emerald-300' />
                    <span className='truncate'>{model}</span>
                  </span>
                ))}
              </div>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
