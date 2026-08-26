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
import { ArrowRight, Check, FileSignature, ReceiptText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { AnimateInView } from '@/components/animate-in-view'
import { EnterpriseContactButton } from '../enterprise-contact-button'

type CTAProps = {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()

  return (
    <>
      <section className='bg-[#f5f8fc] px-4 py-16 text-[#101828] sm:px-6 md:py-20'>
        <AnimateInView className='mx-auto grid max-w-7xl gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(360px,0.72fr)] lg:items-center'>
          <div>
            <p className='text-xs font-semibold text-[#1f63ad]'>
              ENTERPRISE READY
            </p>
            <h2 className='mt-3 max-w-3xl text-3xl leading-tight font-bold md:text-4xl'>
              {t('home.enterprise.title')}
            </h2>
            <p className='mt-4 max-w-2xl text-sm leading-7 text-[#53647a] md:text-base'>
              {t('home.enterprise.description')}
            </p>
            <div className='mt-6 flex flex-wrap gap-x-6 gap-y-3 text-sm font-medium text-[#26374d]'>
              {[
                t('home.enterprise.connected'),
                t('home.enterprise.contract'),
                t('home.enterprise.invoice'),
              ].map((item) => (
                <span key={item} className='flex items-center gap-2'>
                  <Check className='size-4 text-[#14855f]' />
                  {item}
                </span>
              ))}
            </div>
            <EnterpriseContactButton appearance='light' className='mt-7' />
          </div>

          <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-1'>
            <div className='rounded-lg border border-[#d7e2ee] bg-white p-5'>
              <FileSignature className='size-6 text-[#2867ae]' />
              <h3 className='mt-4 text-base font-semibold'>
                {t('home.enterprise.contractTitle')}
              </h3>
              <p className='mt-2 text-sm leading-6 text-[#65758a]'>
                {t('home.enterprise.contractDescription')}
              </p>
            </div>
            <div className='rounded-lg border border-[#d7e2ee] bg-white p-5'>
              <ReceiptText className='size-6 text-[#14855f]' />
              <h3 className='mt-4 text-base font-semibold'>
                {t('home.enterprise.invoiceTitle')}
              </h3>
              <p className='mt-2 text-sm leading-6 text-[#65758a]'>
                {t('home.enterprise.invoiceDescription')}
              </p>
            </div>
          </div>
        </AnimateInView>
      </section>

      <section className='border-y border-white/[0.08] bg-[#0b1a30] px-4 py-14 sm:px-6 md:py-16'>
        <AnimateInView className='mx-auto flex max-w-7xl flex-col justify-between gap-8 md:flex-row md:items-center'>
          <div className='max-w-2xl'>
            <p className='text-xs font-semibold text-cyan-300'>
              READY TO CONNECT
            </p>
            <h2 className='mt-3 text-3xl leading-tight font-bold text-white md:text-4xl'>
              {t('home.cta.title')}
            </h2>
            <div className='mt-4 flex flex-wrap gap-x-5 gap-y-2 text-sm text-white/52'>
              {[
                t('home.cta.item1'),
                t('home.cta.item2'),
                t('home.cta.item3'),
              ].map((item) => (
                <span key={item} className='flex items-center gap-1.5'>
                  <Check className='size-4 text-emerald-300' />
                  {item}
                </span>
              ))}
            </div>
          </div>
          <div className='flex shrink-0 flex-wrap gap-3'>
            <Button
              className='group h-11 rounded-lg bg-white px-5 font-semibold text-[#0b1a30] hover:bg-blue-50'
              render={
                <Link to={props.isAuthenticated ? '/dashboard' : '/sign-up'} />
              }
            >
              {props.isAuthenticated
                ? t('Go to Dashboard')
                : t('home.cta.primaryAction')}
              <ArrowRight className='size-4 transition-transform group-hover:translate-x-0.5' />
            </Button>
            <Button
              variant='outline'
              className='h-11 rounded-lg border-white/20 bg-transparent px-5 text-white hover:bg-white/[0.08] hover:text-white'
              render={<Link to='/pricing' />}
            >
              {t('home.hero.modelsAction')}
            </Button>
          </div>
        </AnimateInView>
      </section>
    </>
  )
}
