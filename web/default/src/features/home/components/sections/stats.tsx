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
import { useTranslation } from 'react-i18next'
import type { HomeModelSummary } from '../../types'

type StatsProps = {
  modelSummary: HomeModelSummary
}

export function Stats({ modelSummary }: StatsProps) {
  const { t } = useTranslation()
  const stats = [
    {
      value: modelSummary.displayTotal,
      label: t('home.stats.models'),
      detail: modelSummary.isLive
        ? t('home.stats.live')
        : t('home.stats.updating'),
    },
    {
      value: '5',
      label: t('home.stats.protocols'),
      detail: 'OpenAI / Anthropic / Gemini',
    },
    {
      value: '24/7',
      label: t('home.stats.monitoring'),
      detail: t('home.stats.routes'),
    },
    {
      value: t('home.stats.enterpriseValue'),
      label: t('home.stats.enterprise'),
      detail: t('home.stats.procurement'),
    },
  ]

  return (
    <section className='border-b border-white/[0.08] bg-[#0a101a]'>
      <div className='mx-auto grid max-w-7xl grid-cols-2 px-4 sm:px-6 md:grid-cols-4'>
        {stats.map((stat, index) => (
          <div
            key={stat.label}
            className={`flex min-h-32 flex-col justify-center py-6 ${index % 2 === 1 ? 'border-l border-white/[0.08] pl-5 sm:pl-8' : 'pr-5 sm:pr-8'} ${index > 1 ? 'border-t border-white/[0.08] md:border-t-0' : ''} ${index > 0 ? 'md:border-l md:border-white/[0.08] md:px-8' : ''}`}
          >
            <strong className='font-mono text-2xl font-bold text-white md:text-3xl'>
              {stat.value}
            </strong>
            <span className='mt-1.5 text-xs font-semibold text-white/75'>
              {stat.label}
            </span>
            <span className='mt-1 text-[10px] leading-4 text-white/35'>
              {stat.detail}
            </span>
          </div>
        ))}
      </div>
    </section>
  )
}
