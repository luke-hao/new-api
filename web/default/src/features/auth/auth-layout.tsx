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
import { Activity, Building2, Check, Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Skeleton } from '@/components/ui/skeleton'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()
  const brandFeatures = [
    { icon: Route, label: t('auth.brand.route') },
    { icon: Activity, label: t('auth.brand.stable') },
    { icon: Building2, label: t('auth.brand.enterprise') },
  ]

  return (
    <div className='relative grid min-h-svh bg-[#07111f] lg:grid-cols-[minmax(360px,0.82fr)_minmax(520px,1.18fr)]'>
      <aside className='relative hidden min-h-svh overflow-hidden border-r border-white/10 px-10 py-9 text-white lg:flex lg:flex-col'>
        <div className='auth-route-grid pointer-events-none absolute inset-0 opacity-45' />
        <Link
          to='/'
          className='relative z-10 flex w-fit items-center gap-3 transition-opacity hover:opacity-80'
        >
          <div className='relative size-9'>
            {loading ? (
              <Skeleton className='absolute inset-0 rounded-lg bg-white/10' />
            ) : (
              <img
                src={logo}
                alt={t('Logo')}
                className='size-9 rounded-lg object-contain'
              />
            )}
          </div>
          {loading ? (
            <Skeleton className='h-6 w-24 bg-white/10' />
          ) : (
            <span className='text-lg font-semibold'>{systemName}</span>
          )}
        </Link>

        <div className='relative z-10 my-auto max-w-lg py-12'>
          <div className='mb-7 flex size-20 items-center justify-center rounded-lg border border-white/12 bg-white/[0.05] shadow-[0_20px_60px_rgba(0,0,0,0.32)]'>
            <img src='/logo.svg' alt='' className='size-14' />
          </div>
          <p className='text-xs font-semibold text-cyan-300'>
            UNIFIED AI ROUTING
          </p>
          <h1 className='mt-4 max-w-md text-4xl leading-tight font-bold'>
            {t('auth.brand.title')}
          </h1>
          <p className='mt-5 max-w-md text-sm leading-7 text-white/55'>
            {t('auth.brand.description')}
          </p>
          <div className='mt-8 grid gap-3 text-sm text-white/72'>
            {brandFeatures.map((feature) => {
              const FeatureIcon = feature.icon
              return (
                <div key={feature.label} className='flex items-center gap-3'>
                  <span className='flex size-8 items-center justify-center rounded-lg border border-white/10 bg-white/[0.05] text-cyan-300'>
                    <FeatureIcon className='size-4' />
                  </span>
                  <span className='flex items-center gap-2'>
                    {feature.label}
                    <Check className='size-3.5 text-emerald-300' />
                  </span>
                </div>
              )
            })}
          </div>
        </div>
        <p className='relative z-10 text-xs text-white/35'>
          {t('auth.brand.footer')}
        </p>
      </aside>

      <section className='auth-form-surface flex min-h-svh flex-col bg-[#f7f9fc] text-[#101828]'>
        <div className='flex h-16 items-center border-b border-[#dfe7f1] px-5 lg:hidden'>
          <Link to='/' className='flex items-center gap-2.5'>
            <img src={logo} alt={t('Logo')} className='size-8 rounded-lg' />
            <span className='font-semibold'>{systemName}</span>
          </Link>
        </div>
        <div className='flex flex-1 items-center justify-center overflow-y-auto px-5 py-10 sm:px-10'>
          <div className='w-full max-w-[440px]'>{children}</div>
        </div>
      </section>
    </div>
  )
}
