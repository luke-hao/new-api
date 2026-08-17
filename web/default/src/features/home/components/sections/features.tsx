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
import {
  Bot,
  Check,
  Code2,
  Cpu,
  ImageIcon,
  Sparkles,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

interface FeaturesProps {
  className?: string
}

interface ModelOffer {
  name: string
  eyebrow: string
  models: string[]
  rate: string
  unitKey: string
  descriptionKey: string
  icon: LucideIcon
  iconClassName: string
  borderClassName: string
  highlighted?: boolean
}

const MODEL_OFFERS: ModelOffer[] = [
  {
    name: 'Codex',
    eyebrow: 'GPT-5.6 / OPENAI CODE',
    models: ['GPT-5.6', 'GPT-5.5', 'Responses API'],
    rate: '0.06x',
    unitKey: 'home.models.ratioFrom',
    descriptionKey: 'home.models.codexDescription',
    icon: Code2,
    iconClassName: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
    borderClassName: 'border-t-emerald-500',
    highlighted: true,
  },
  {
    name: 'Claude',
    eyebrow: 'ANTHROPIC ROUTE',
    models: ['fable-5', 'sonnet-5', 'claude-opus-4-8'],
    rate: '0.25x',
    unitKey: 'home.models.ratioFrom',
    descriptionKey: 'home.models.claudeDescription',
    icon: Bot,
    iconClassName: 'bg-orange-500/10 text-orange-600 dark:text-orange-400',
    borderClassName: 'border-t-orange-500',
  },
  {
    name: 'CCMAX',
    eyebrow: 'FULL POWER',
    models: ['official direct', 'platform checks', 'full version'],
    rate: '0.95x',
    unitKey: 'home.models.ratioFrom',
    descriptionKey: 'home.models.ccmaxDescription',
    icon: Cpu,
    iconClassName: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
    borderClassName: 'border-t-blue-500',
  },
  {
    name: 'gpt-image-2',
    eyebrow: 'IMAGE GENERATION',
    models: ['text to image', 'high quality', 'API ready'],
    rate: '0.09',
    unitKey: 'home.models.perImageFrom',
    descriptionKey: 'home.models.imageDescription',
    icon: ImageIcon,
    iconClassName: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
    borderClassName: 'border-t-rose-500',
  },
  {
    name: 'Gemini',
    eyebrow: 'GOOGLE AI',
    models: ['3.5-flash', '3.1-pro', 'multimodal'],
    rate: '0.5x',
    unitKey: 'home.models.ratioFrom',
    descriptionKey: 'home.models.geminiDescription',
    icon: Sparkles,
    iconClassName: 'bg-violet-500/10 text-violet-600 dark:text-violet-400',
    borderClassName: 'border-t-violet-500',
  },
]

export function Features(_props: FeaturesProps) {
  const { t } = useTranslation()

  return (
    <section className='relative z-10 px-4 py-20 sm:px-6 md:py-28'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-12 flex flex-col justify-between gap-5 md:flex-row md:items-end'>
          <div className='max-w-2xl'>
            <p className='mb-3 text-xs font-semibold text-emerald-600 dark:text-emerald-400'>
              MODEL CATALOG / {t('home.models.available')}
            </p>
            <h2 className='text-3xl leading-tight font-bold md:text-4xl'>
              {t('home.models.title')}
            </h2>
          </div>
          <p className='text-muted-foreground max-w-md text-sm leading-6 md:text-right'>
            {t('home.models.description')}
          </p>
        </AnimateInView>

        <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-6'>
          {MODEL_OFFERS.map((offer, index) => {
            const Icon = offer.icon
            return (
              <AnimateInView
                key={offer.name}
                delay={index * 80}
                animation='fade-up'
                className={`group min-h-64 rounded-lg border border-t-2 p-6 transition-colors ${offer.borderClassName} ${
                  offer.highlighted
                    ? 'border-emerald-500/40 bg-emerald-500/[0.04] shadow-[0_16px_40px_-28px_rgba(16,185,129,0.55)] hover:bg-emerald-500/[0.07]'
                    : 'border-border/60 bg-background hover:bg-muted/20'
                } ${index < 3 ? 'lg:col-span-2' : 'lg:col-span-3'}`}
              >
                <div className='flex items-start justify-between gap-4'>
                  <span
                    className={`flex size-10 items-center justify-center rounded-lg ${offer.iconClassName}`}
                  >
                    <Icon
                      className='size-5'
                      strokeWidth={1.8}
                      aria-hidden='true'
                    />
                  </span>
                  {offer.highlighted ? (
                    <span className='rounded-full border border-emerald-500/25 bg-emerald-500/8 px-2 py-1 text-[10px] font-bold text-emerald-600 dark:text-emerald-400'>
                      {t('home.models.hot')}
                    </span>
                  ) : null}
                </div>

                <p className='text-muted-foreground mt-5 text-[10px] font-semibold'>
                  {offer.eyebrow}
                </p>
                <h3 className='mt-1 text-xl font-bold'>{offer.name}</h3>
                <p className='text-muted-foreground mt-2 min-h-10 text-xs leading-5'>
                  {t(offer.descriptionKey)}
                </p>

                <div className='mt-5 flex flex-wrap gap-1.5'>
                  {offer.models.map((model) => (
                    <span
                      key={model}
                      className='bg-muted text-muted-foreground inline-flex items-center gap-1 rounded-md px-2 py-1 font-mono text-[10px]'
                    >
                      <Check
                        className='size-3 text-emerald-600'
                        aria-hidden='true'
                      />
                      {model}
                    </span>
                  ))}
                </div>

                <div className='border-border/50 mt-6 flex items-end justify-between border-t pt-4'>
                  <span className='text-muted-foreground text-xs'>
                    {t(offer.unitKey)}
                  </span>
                  <strong
                    className={`font-mono font-bold ${
                      offer.highlighted
                        ? 'text-3xl text-emerald-600 dark:text-emerald-400'
                        : 'text-2xl'
                    }`}
                  >
                    {offer.rate}
                  </strong>
                </div>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
