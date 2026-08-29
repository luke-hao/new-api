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
import { Check, FileSignature, ReceiptText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'
import { EnterpriseContactButton } from '../enterprise-contact-button'

type CTAProps = {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(_props: CTAProps) {
  const { t } = useTranslation()

  return (
    <section className='bg-[#f5f8fc] px-4 py-14 text-[#101828] sm:px-6 md:py-16'>
      <AnimateInView className='mx-auto grid max-w-7xl gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(360px,0.72fr)] lg:items-center'>
        <div>
          <p className='text-xs font-semibold text-[#1f63ad]'>
            ENTERPRISE READY
          </p>
          <h2 className='mt-3 max-w-3xl text-2xl leading-tight font-bold md:text-3xl'>
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
          <EnterpriseContactButton appearance='light' className='mt-6' />
        </div>

        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-1'>
          <div className='rounded-lg border border-[#d7e2ee] bg-white p-4'>
            <FileSignature className='size-6 text-[#2867ae]' />
            <h3 className='mt-4 text-base font-semibold'>
              {t('home.enterprise.contractTitle')}
            </h3>
            <p className='mt-2 text-sm leading-6 text-[#65758a]'>
              {t('home.enterprise.contractDescription')}
            </p>
          </div>
          <div className='rounded-lg border border-[#d7e2ee] bg-white p-4'>
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
  )
}
