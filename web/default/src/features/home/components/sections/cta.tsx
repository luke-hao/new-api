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
import { ArrowRight, Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { AnimateInView } from '@/components/animate-in-view'

interface CTAProps {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()

  return (
    <section className='relative z-10 bg-[#0b0d0c] px-4 py-16 text-white sm:px-6 md:py-20'>
      <AnimateInView className='mx-auto flex max-w-6xl flex-col justify-between gap-8 md:flex-row md:items-center'>
        <div className='max-w-2xl'>
          <p className='text-xs font-semibold text-emerald-400'>
            READY TO CONNECT
          </p>
          <h2 className='mt-3 text-3xl leading-tight font-bold md:text-4xl'>
            {t('home.cta.title')}
          </h2>
          <div className='mt-4 flex flex-wrap gap-x-5 gap-y-2 text-sm text-white/55'>
            {[
              t('home.cta.item1'),
              t('home.cta.item2'),
              t('home.cta.item3'),
            ].map((item) => (
              <span key={item} className='flex items-center gap-1.5'>
                <Check className='size-4 text-emerald-400' aria-hidden='true' />
                {item}
              </span>
            ))}
          </div>
        </div>

        <div className='flex shrink-0 flex-wrap gap-3'>
          <Button
            className='group h-11 rounded-lg bg-white px-5 font-semibold text-black hover:bg-white/90'
            render={
              <Link to={props.isAuthenticated ? '/dashboard' : '/sign-up'} />
            }
          >
            {props.isAuthenticated
              ? t('Go to Dashboard')
              : t('home.cta.primaryAction')}
            <ArrowRight
              className='size-4 transition-transform group-hover:translate-x-0.5'
              aria-hidden='true'
            />
          </Button>
          <Button
            variant='outline'
            className='h-11 rounded-lg border-white/20 bg-transparent px-5 text-white hover:bg-white/8 hover:text-white'
            render={<Link to='/pricing' />}
          >
            {t('View Pricing')}
          </Button>
        </div>
      </AnimateInView>
    </section>
  )
}
