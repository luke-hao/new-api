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
import { useEffect, useState, type ComponentType } from 'react'
import {
  Bot,
  Check,
  Code2,
  Cpu,
  ImageIcon,
  Sparkles,
  type LucideProps,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

type ResourceTone = 'orange' | 'blue' | 'emerald' | 'rose' | 'violet'

interface ResourceItem {
  id: string
  name: string
  models: string
  rate: string
  detailKey: string
  tone: ResourceTone
  icon: ComponentType<LucideProps>
}

const TONE_CLASSES: Record<
  ResourceTone,
  { icon: string; active: string; rate: string }
> = {
  orange: {
    icon: 'bg-orange-400/12 text-orange-300',
    active: 'border-orange-400/35 bg-orange-400/8',
    rate: 'text-orange-300',
  },
  blue: {
    icon: 'bg-blue-400/12 text-blue-300',
    active: 'border-blue-400/35 bg-blue-400/8',
    rate: 'text-blue-300',
  },
  emerald: {
    icon: 'bg-emerald-400/12 text-emerald-300',
    active: 'border-emerald-400/35 bg-emerald-400/8',
    rate: 'text-emerald-300',
  },
  rose: {
    icon: 'bg-rose-400/12 text-rose-300',
    active: 'border-rose-400/35 bg-rose-400/8',
    rate: 'text-rose-300',
  },
  violet: {
    icon: 'bg-violet-400/12 text-violet-300',
    active: 'border-violet-400/35 bg-violet-400/8',
    rate: 'text-violet-300',
  },
}

const RESOURCES: ResourceItem[] = [
  {
    id: 'codex',
    name: 'Codex',
    models: 'GPT-5.6 / GPT-5.5',
    rate: '0.06x',
    detailKey: 'home.pool.detail.codex',
    tone: 'emerald',
    icon: Code2,
  },
  {
    id: 'claude',
    name: 'Claude',
    models: 'fable-5 / sonnet-5 / opus-4-8',
    rate: '0.25x',
    detailKey: 'home.pool.detail.claude',
    tone: 'orange',
    icon: Bot,
  },
  {
    id: 'ccmax',
    name: 'CCMAX',
    models: 'MAX FULL POWER',
    rate: '0.95x',
    detailKey: 'home.pool.detail.ccmax',
    tone: 'blue',
    icon: Cpu,
  },
  {
    id: 'image',
    name: 'gpt-image-2',
    models: 'IMAGE GENERATION',
    rate: '0.09',
    detailKey: 'home.pool.detail.image',
    tone: 'rose',
    icon: ImageIcon,
  },
  {
    id: 'gemini',
    name: 'Gemini',
    models: '3.5-flash / 3.1-pro',
    rate: '0.5x',
    detailKey: 'home.pool.detail.gemini',
    tone: 'violet',
    icon: Sparkles,
  },
]

interface HeroTerminalDemoProps {
  className?: string
}

export function HeroTerminalDemo(props: HeroTerminalDemoProps) {
  const { t } = useTranslation()
  const [activeIndex, setActiveIndex] = useState(0)
  const activeResource = RESOURCES[activeIndex]
  const activeTone = TONE_CLASSES[activeResource.tone]

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    if (mediaQuery.matches) return

    const intervalId = window.setInterval(() => {
      setActiveIndex((current) => (current + 1) % RESOURCES.length)
    }, 3600)

    return () => window.clearInterval(intervalId)
  }, [])

  return (
    <div className={cn('w-full max-w-[520px]', props.className)}>
      <div className='overflow-hidden rounded-lg border border-white/10 bg-[#0b0d0c] text-white shadow-[0_24px_70px_-28px_rgba(0,0,0,0.55)]'>
        <div className='flex items-center justify-between border-b border-white/8 px-5 py-4'>
          <div>
            <p className='text-[10px] font-semibold text-white/40'>
              LIVE RESOURCE POOL
            </p>
            <p className='mt-1 text-sm font-semibold'>{t('home.pool.title')}</p>
          </div>
          <div className='flex items-center gap-2 rounded-full border border-emerald-400/20 bg-emerald-400/8 px-2.5 py-1 text-[11px] font-semibold text-emerald-300'>
            <span className='size-1.5 rounded-full bg-emerald-400 shadow-[0_0_10px_rgba(52,211,153,0.8)] motion-safe:animate-pulse' />
            {t('home.pool.operational')}
          </div>
        </div>

        <div className='px-3 py-3'>
          {RESOURCES.map((resource, index) => {
            const Icon = resource.icon
            const tone = TONE_CLASSES[resource.tone]
            const isActive = index === activeIndex

            return (
              <button
                key={resource.id}
                type='button'
                aria-pressed={isActive}
                onClick={() => setActiveIndex(index)}
                className={cn(
                  'grid min-h-16 w-full grid-cols-[2.25rem_minmax(0,1fr)_auto] items-center gap-3 rounded-md border border-transparent px-3 py-2 text-left transition-colors',
                  isActive ? tone.active : 'hover:bg-white/[0.04]'
                )}
              >
                <span
                  className={cn(
                    'flex size-9 items-center justify-center rounded-md',
                    tone.icon
                  )}
                >
                  <Icon
                    className='size-4.5'
                    strokeWidth={1.8}
                    aria-hidden='true'
                  />
                </span>
                <span className='min-w-0'>
                  <span className='block text-sm font-semibold'>
                    {resource.name}
                  </span>
                  <span className='block truncate font-mono text-[10px] text-white/42'>
                    {resource.models}
                  </span>
                </span>
                <span className='text-right'>
                  <span
                    className={cn(
                      'block font-mono text-base font-bold',
                      tone.rate
                    )}
                  >
                    {resource.rate}
                  </span>
                  <span className='block text-[9px] font-medium text-white/35'>
                    {resource.id === 'image'
                      ? t('home.pool.perImage')
                      : t('home.pool.from')}
                  </span>
                </span>
              </button>
            )
          })}
        </div>

        <div className='border-t border-white/8 bg-white/[0.025] px-5 py-4'>
          <div className='flex items-start gap-3'>
            <span
              className={cn(
                'mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full',
                activeTone.icon
              )}
            >
              <Check
                className='size-3.5'
                strokeWidth={2.5}
                aria-hidden='true'
              />
            </span>
            <div className='min-w-0'>
              <p className='text-xs font-semibold text-white/88'>
                {activeResource.name} · {t('home.pool.ready')}
              </p>
              <p className='mt-1 text-xs leading-5 text-white/45'>
                {t(activeResource.detailKey)}
              </p>
            </div>
          </div>
        </div>

        <div className='grid grid-cols-3 border-t border-white/8'>
          {[
            ['< 5s', t('home.pool.firstToken')],
            ['24/7', t('home.pool.monitoring')],
            ['1st', t('home.pool.directSupply')],
          ].map(([value, label], index) => (
            <div
              key={label}
              className={cn(
                'px-4 py-3 text-center',
                index > 0 && 'border-l border-white/8'
              )}
            >
              <p className='font-mono text-sm font-bold text-white/90'>
                {value}
              </p>
              <p className='mt-0.5 text-[9px] font-medium text-white/35'>
                {label}
              </p>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
