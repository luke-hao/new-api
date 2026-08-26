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
import { ArrowRight, BookOpen, Check, CircleDot } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import type { HomeModelSummary } from '../../types'
import { EnterpriseContactButton } from '../enterprise-contact-button'
import { HeroTerminalDemo } from '../hero-terminal-demo'

type HeroProps = {
  className?: string
  isAuthenticated?: boolean
  modelSummary: HomeModelSummary
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const proofPoints = [
    t('home.hero.proof.enterprise'),
    t('home.hero.proof.contract'),
    t('home.hero.proof.invoice'),
    t('home.hero.proof.pool'),
  ]

  return (
    <section className='relative isolate flex h-[calc(100svh-5rem)] max-h-[780px] min-h-[680px] items-center overflow-hidden border-b border-white/[0.08] px-4 pt-20 pb-10 sm:px-6'>
      <HeroTerminalDemo modelSummary={props.modelSummary} />
      <div className='relative z-10 mx-auto w-full max-w-7xl'>
        <div className='max-w-[650px]'>
          <div
            className='landing-animate-fade-up mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-300/20 bg-emerald-300/[0.07] px-3 py-1.5 text-xs font-semibold text-emerald-200 opacity-0'
            style={{ animationDelay: '0ms' }}
          >
            <span className='relative flex size-2' aria-hidden='true'>
              <span className='absolute inline-flex size-full rounded-full bg-emerald-400 opacity-40 motion-safe:animate-ping' />
              <span className='relative inline-flex size-2 rounded-full bg-emerald-400' />
            </span>
            {t('home.hero.badge')}
          </div>

          <p
            className='landing-animate-fade-up mb-3 flex items-center gap-2 text-xs font-semibold text-blue-200/75 opacity-0'
            style={{ animationDelay: '45ms' }}
          >
            <CircleDot className='size-4 text-cyan-300' aria-hidden='true' />
            {t('home.hero.eyebrow')}
          </p>

          <h1
            className='landing-animate-fade-up text-5xl leading-[1.05] font-bold text-white opacity-0 md:text-7xl'
            style={{ animationDelay: '90ms' }}
          >
            {t('home.hero.brand')}
          </h1>
          <p
            className='landing-animate-fade-up mt-4 max-w-xl text-2xl leading-tight font-semibold text-white/92 opacity-0 md:text-4xl'
            style={{ animationDelay: '135ms' }}
          >
            {t('home.hero.titlePrefix')}{' '}
            <span className='text-cyan-300'>
              {props.modelSummary.displayTotal}
            </span>{' '}
            {t('home.hero.titleSuffix')}
          </p>
          <p
            className='landing-animate-fade-up mt-5 max-w-xl text-sm leading-7 text-white/58 opacity-0 md:text-base'
            style={{ animationDelay: '180ms' }}
          >
            {t('home.hero.description')}
          </p>

          <div
            className='landing-animate-fade-up mt-6 grid max-w-xl grid-cols-2 gap-x-5 gap-y-3 opacity-0'
            style={{ animationDelay: '220ms' }}
          >
            {proofPoints.map((item) => (
              <div
                key={item}
                className='flex min-w-0 items-center gap-2 text-xs font-medium text-white/72 sm:text-sm'
              >
                <span className='flex size-5 shrink-0 items-center justify-center rounded-full bg-blue-400/12 text-cyan-300'>
                  <Check className='size-3.5' strokeWidth={2.5} />
                </span>
                <span>{item}</span>
              </div>
            ))}
          </div>

          <div
            className='landing-animate-fade-up mt-8 flex flex-wrap items-center gap-3 opacity-0'
            style={{ animationDelay: '270ms' }}
          >
            <Button
              className='group h-11 rounded-lg bg-[#4f8cff] px-5 text-sm font-semibold text-white hover:bg-[#6a9cff]'
              render={
                <Link to={props.isAuthenticated ? '/dashboard' : '/sign-up'} />
              }
            >
              {props.isAuthenticated
                ? t('Go to Dashboard')
                : t('home.hero.primaryAction')}
              <ArrowRight className='size-4 transition-transform group-hover:translate-x-0.5' />
            </Button>
            <Button
              variant='outline'
              className='h-11 rounded-lg border-white/18 bg-white/[0.04] px-5 text-sm font-medium text-white hover:bg-white/[0.09] hover:text-white'
              render={<Link to='/pricing' />}
            >
              {t('home.hero.modelsAction')}
            </Button>
            <Button
              variant='ghost'
              className='h-11 rounded-lg px-4 text-sm text-white/65 hover:bg-white/[0.07] hover:text-white'
              render={
                <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
              }
            >
              <BookOpen className='size-4' />
              {t('Docs')}
            </Button>
          </div>
          <EnterpriseContactButton className='mt-4' />
        </div>
      </div>
    </section>
  )
}
