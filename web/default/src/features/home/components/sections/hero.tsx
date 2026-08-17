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
import { ArrowRight, BookOpen, Check, FileCheck2, Flame } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { HeroTerminalDemo } from '../hero-terminal-demo'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const proofPoints = [
    t('home.hero.proof.invoice'),
    t('home.hero.proof.lowRatio'),
    t('home.hero.proof.lowLatency'),
    t('home.hero.proof.ownedPool'),
  ]

  const docsButton = docsUrl.startsWith('http') ? (
    <Button
      variant='outline'
      className='h-11 rounded-lg px-4 text-sm font-medium'
      render={<a href={docsUrl} target='_blank' rel='noopener noreferrer' />}
    >
      <BookOpen className='size-4' aria-hidden='true' />
      {t('Docs')}
    </Button>
  ) : (
    <Button
      variant='outline'
      className='h-11 rounded-lg px-4 text-sm font-medium'
      render={<Link to={docsUrl} />}
    >
      <BookOpen className='size-4' aria-hidden='true' />
      {t('Docs')}
    </Button>
  )

  return (
    <section className='border-border/50 relative z-10 overflow-hidden border-b px-4 pt-24 pb-12 sm:px-6 md:pt-28 md:pb-16'>
      <div className='mx-auto grid max-w-6xl items-center gap-12 lg:grid-cols-[minmax(0,1fr)_minmax(480px,0.92fr)] lg:gap-14'>
        <div className='flex flex-col items-start'>
          <div
            className='landing-animate-fade-up mb-5 inline-flex items-center gap-2 rounded-full border border-emerald-500/30 bg-emerald-500/8 px-3 py-1.5 text-xs font-semibold text-emerald-700 opacity-0 dark:text-emerald-300'
            style={{ animationDelay: '0ms' }}
          >
            <span className='relative flex size-2' aria-hidden='true'>
              <span className='absolute inline-flex size-full rounded-full bg-emerald-500 opacity-50 motion-safe:animate-ping' />
              <span className='relative inline-flex size-2 rounded-full bg-emerald-500' />
            </span>
            {t('home.hero.badge')}
          </div>

          <p
            className='landing-animate-fade-up text-muted-foreground mb-3 flex items-center gap-2 text-xs font-semibold opacity-0'
            style={{ animationDelay: '40ms' }}
          >
            <Flame className='size-4 text-orange-500' aria-hidden='true' />
            AI MODEL SUPPLY / 2026
          </p>

          <h1
            className='landing-animate-fade-up max-w-2xl text-4xl leading-[1.08] font-bold opacity-0 md:text-5xl lg:text-6xl'
            style={{ animationDelay: '80ms' }}
          >
            {t('home.hero.titleLine1')}
            <span className='mt-1 block text-emerald-600 dark:text-emerald-400'>
              {t('home.hero.titleLine2')}
            </span>
          </h1>

          <p
            className='landing-animate-fade-up text-muted-foreground mt-5 max-w-xl text-base leading-7 opacity-0 md:text-lg'
            style={{ animationDelay: '140ms' }}
          >
            {t('home.hero.description')}
          </p>

          <div
            className='landing-animate-fade-up mt-6 grid w-full max-w-xl grid-cols-2 gap-x-5 gap-y-3 opacity-0'
            style={{ animationDelay: '190ms' }}
          >
            {proofPoints.map((item) => (
              <div
                key={item}
                className='flex min-w-0 items-center gap-2 text-sm font-medium'
              >
                <span className='flex size-5 shrink-0 items-center justify-center rounded-full bg-emerald-500/12 text-emerald-600 dark:text-emerald-400'>
                  <Check
                    className='size-3.5'
                    strokeWidth={2.5}
                    aria-hidden='true'
                  />
                </span>
                <span>{item}</span>
              </div>
            ))}
          </div>

          <div
            className='landing-animate-fade-up mt-8 flex w-full flex-wrap items-center gap-3 opacity-0'
            style={{ animationDelay: '240ms' }}
          >
            <Button
              className='group bg-foreground text-background hover:bg-foreground/90 h-11 rounded-lg px-5 text-sm font-semibold'
              render={
                <Link to={props.isAuthenticated ? '/dashboard' : '/sign-up'} />
              }
            >
              {props.isAuthenticated
                ? t('Go to Dashboard')
                : t('home.hero.primaryAction')}
              <ArrowRight
                className='size-4 transition-transform group-hover:translate-x-0.5'
                aria-hidden='true'
              />
            </Button>
            <Button
              variant='outline'
              className='h-11 rounded-lg px-5 text-sm font-medium'
              render={<Link to='/pricing' />}
            >
              {t('View Pricing')}
            </Button>
            {docsButton}
          </div>

          <div
            className='landing-animate-fade-up text-muted-foreground mt-7 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs opacity-0'
            style={{ animationDelay: '290ms' }}
          >
            <span className='text-foreground flex items-center gap-1.5 font-medium'>
              <FileCheck2
                className='size-4 text-emerald-600'
                aria-hidden='true'
              />
              {t('home.hero.flowVerification')}
            </span>
            <span>Claude</span>
            <span>Codex</span>
            <span>Gemini</span>
            <span>gpt-image-2</span>
          </div>
        </div>

        <div
          className='landing-animate-fade-left flex w-full justify-center opacity-0 lg:justify-end'
          style={{ animationDelay: '180ms' }}
        >
          <HeroTerminalDemo />
        </div>
      </div>
    </section>
  )
}
