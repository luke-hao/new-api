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
import { Activity, Route, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { DEFAULT_LOGO } from '@/lib/constants'
import type { HomeModelSummary } from '../types'
import { LazyLobeIcon } from './lazy-lobe-icon'

const NODE_POSITIONS = [
  { left: '57%', top: '16%' },
  { left: '77%', top: '26%' },
  { left: '82%', top: '57%' },
  { left: '62%', top: '72%' },
  { left: '42%', top: '61%' },
  { left: '38%', top: '31%' },
]

type HeroTerminalDemoProps = {
  modelSummary: HomeModelSummary
}

export function HeroTerminalDemo({ modelSummary }: HeroTerminalDemoProps) {
  const { t } = useTranslation()

  return (
    <div
      className='pointer-events-none absolute inset-0 overflow-hidden'
      aria-hidden='true'
    >
      <div className='absolute inset-y-0 right-0 w-[72%] opacity-70 max-lg:w-full max-lg:opacity-35'>
        {[18, 36, 54, 72, 90].map((top) => (
          <span
            key={`row-${top}`}
            className='absolute right-0 left-0 h-px bg-white/[0.055]'
            style={{ top: `${top}%` }}
          />
        ))}
        {[18, 36, 54, 72, 90].map((left) => (
          <span
            key={`column-${left}`}
            className='absolute top-0 bottom-0 w-px bg-white/[0.055]'
            style={{ left: `${left}%` }}
          />
        ))}

        <div className='absolute top-1/2 left-[62%] size-40 -translate-x-1/2 -translate-y-1/2 max-md:top-[20%] max-md:left-[84%] max-md:size-24'>
          <span className='landing-route-orbit absolute inset-0 rounded-full border border-blue-400/20' />
          <span className='landing-route-orbit-reverse absolute inset-5 rounded-full border border-cyan-300/20' />
          <span className='absolute inset-10 flex items-center justify-center rounded-full border border-white/15 bg-[#091526] shadow-[0_18px_60px_rgba(0,0,0,0.5)] max-md:inset-7'>
            <img
              src={DEFAULT_LOGO}
              alt=''
              className='h-20 w-auto max-w-none object-contain max-md:h-14'
            />
          </span>
        </div>

        {modelSummary.families.map((family, index) => (
          <div
            key={family.id}
            className='landing-model-node absolute flex min-w-36 items-center gap-3 rounded-lg border border-white/10 bg-[#0b1422]/95 px-3 py-2.5 shadow-[0_14px_40px_rgba(0,0,0,0.35)] max-md:hidden'
            style={NODE_POSITIONS[index]}
          >
            <span className='flex size-8 shrink-0 items-center justify-center rounded-md bg-white/[0.06]'>
              <LazyLobeIcon iconName={family.iconName} size={19} />
            </span>
            <span className='min-w-0 max-md:hidden'>
              <span className='block text-xs font-semibold text-white'>
                {family.title}
              </span>
              <span className='mt-0.5 block max-w-28 truncate text-[10px] text-white/40'>
                {family.models[0]}
              </span>
            </span>
            <span className='ml-auto size-1.5 shrink-0 rounded-full bg-emerald-400 shadow-[0_0_10px_rgba(52,211,153,0.75)]' />
          </div>
        ))}

        <div className='absolute right-[7%] bottom-[8%] hidden items-center gap-5 text-[10px] font-medium text-white/42 xl:flex'>
          <span className='flex items-center gap-1.5'>
            <Route className='size-3.5 text-blue-300' />
            {t('home.network.route')}
          </span>
          <span className='flex items-center gap-1.5'>
            <Activity className='size-3.5 text-cyan-300' />
            {t('home.network.monitor')}
          </span>
          <span className='flex items-center gap-1.5'>
            <ShieldCheck className='size-3.5 text-emerald-300' />
            {t('home.network.stable')}
          </span>
        </div>
      </div>
    </div>
  )
}
