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
import { useCallback, useRef, useState } from 'react'
import {
  ExternalLink,
  Gift,
  Loader2,
  ShoppingBag,
  TicketCheck,
  TriangleAlert,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { SectionPageLayout } from '@/components/layout'
import { useRedemption, useTopupInfo } from '@/features/wallet/hooks'
import { RechargeGuide } from './components/recharge-guide'

type RechargeProps = {
  topupLinkOverride?: string
  title?: string
}

export function Recharge({
  topupLinkOverride,
  title = 'Recharge',
}: RechargeProps = {}) {
  const { t } = useTranslation()
  const [redemptionCode, setRedemptionCode] = useState('')
  const redemptionInputRef = useRef<HTMLInputElement>(null)
  const purchaseSectionRef = useRef<HTMLDivElement>(null)
  const { topupInfo, loading: topupInfoLoading } = useTopupInfo()
  const { redeeming, redeemCode } = useRedemption()
  const setUser = useAuthStore((state) => state.auth.setUser)
  const userId = useAuthStore((state) => state.auth.user?.id)
  const topupLink = topupLinkOverride?.trim() || topupInfo?.topup_link?.trim()
  const loading = topupLinkOverride ? false : topupInfoLoading
  const redemptionEnabled = topupInfo?.enable_redemption !== false
  const pageTitle = t(title)

  const getScrollBehavior = useCallback((): ScrollBehavior => {
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches
      ? 'auto'
      : 'smooth'
  }, [])

  const goToPurchase = useCallback(() => {
    purchaseSectionRef.current?.scrollIntoView({
      behavior: getScrollBehavior(),
      block: 'start',
    })
  }, [getScrollBehavior])

  const goToRedeem = useCallback(() => {
    const input = redemptionInputRef.current
    if (!input) return

    input.scrollIntoView({
      behavior: getScrollBehavior(),
      block: 'center',
    })
    input.focus({ preventScroll: true })
  }, [getScrollBehavior])

  const handleRedeem = async () => {
    const success = await redeemCode(redemptionCode)
    if (!success) return

    setRedemptionCode('')
    const response = await getSelf()
    if (response.success && response.data) {
      setUser(response.data as AuthUser)
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{pageTitle}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4'>
          {!topupInfoLoading && redemptionEnabled && (
            <RechargeGuide
              key={userId}
              userId={userId}
              onGoToPurchase={goToPurchase}
              onGoToRedeem={goToRedeem}
            />
          )}

          <TitledCard
            title={t('Redeem your purchased card code')}
            description={t(
              'Paste the card code copied from the order page below.'
            )}
            icon={<Gift className='h-4 w-4' />}
            disableHoverEffect
            contentClassName='space-y-3'
          >
            {redemptionEnabled ? (
              <>
                <div className='flex items-center gap-2'>
                  <Gift className='text-muted-foreground h-4 w-4' />
                  <Label
                    htmlFor='recharge-redemption-code'
                    className='text-muted-foreground text-xs font-medium tracking-wider uppercase'
                  >
                    {t('Card code')}
                  </Label>
                </div>
                <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
                  <Input
                    ref={redemptionInputRef}
                    id='recharge-redemption-code'
                    value={redemptionCode}
                    onChange={(event) => setRedemptionCode(event.target.value)}
                    onKeyDown={(event) => {
                      if (
                        event.key === 'Enter' &&
                        !event.nativeEvent.isComposing &&
                        !redeeming
                      ) {
                        void handleRedeem()
                      }
                    }}
                    placeholder={t(
                      'Paste the card code here, not the order number'
                    )}
                    className='h-9 min-w-0'
                  />
                  <Button
                    onClick={handleRedeem}
                    disabled={redeeming}
                    variant='outline'
                    className='h-9 px-4'
                  >
                    {redeeming && (
                      <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                    )}
                    {t('Redeem')}
                  </Button>
                </div>
                <Alert className='border-amber-500/40 bg-amber-500/10 py-2'>
                  <TriangleAlert className='text-amber-700 dark:text-amber-400' />
                  <AlertDescription className='text-xs leading-5 text-amber-900 dark:text-amber-200'>
                    {t(
                      'Remove a trailing amount marker such as ---------10$ before redeeming. For example, enter abcdef instead of abcdef---------10$.'
                    )}
                  </AlertDescription>
                </Alert>
                {topupLink && (
                  <p className='text-muted-foreground text-xs'>
                    {t('Need a redemption code?')}{' '}
                    <a
                      href={topupLink}
                      target='_blank'
                      rel='noopener noreferrer'
                      className='inline-flex items-center gap-1 underline-offset-4 hover:underline'
                    >
                      {t('Get one here')}
                      <ExternalLink className='h-3 w-3' />
                    </a>
                  </p>
                )}
              </>
            ) : (
              <Alert>
                <AlertDescription>
                  {t(
                    'Redemption codes are disabled until the administrator confirms compliance terms.'
                  )}
                </AlertDescription>
              </Alert>
            )}
          </TitledCard>

          <div ref={purchaseSectionRef} className='scroll-mt-4'>
            <TitledCard
              title={t('Buy a card code')}
              description={t(
                'After payment, copy the card code and return here to redeem it.'
              )}
              icon={<ShoppingBag className='h-4 w-4' />}
              action={
                topupLink ? (
                  <div className='flex w-full flex-col gap-2 sm:w-auto sm:flex-row'>
                    {redemptionEnabled && (
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='w-full gap-2 sm:w-auto'
                        onClick={goToRedeem}
                      >
                        <TicketCheck className='h-4 w-4' />
                        {t('I have a card code')}
                      </Button>
                    )}
                    <Button
                      variant='outline'
                      size='sm'
                      className='w-full gap-2 sm:w-auto'
                      render={
                        <a
                          href={topupLink}
                          target='_blank'
                          rel='noopener noreferrer'
                        />
                      }
                    >
                      <ExternalLink className='h-4 w-4' />
                      {t('Open in new tab')}
                    </Button>
                  </div>
                ) : null
              }
              disableHoverEffect
              contentClassName='min-h-0 p-0'
            >
              {loading ? (
                <div className='space-y-3 p-3 sm:p-5'>
                  <Skeleton className='h-10 w-full' />
                  <Skeleton className='h-[52vh] w-full rounded-lg' />
                </div>
              ) : topupLink ? (
                <iframe
                  title={pageTitle}
                  src={topupLink}
                  className='bg-background h-[70vh] min-h-[520px] w-full border-0'
                  referrerPolicy='no-referrer-when-downgrade'
                />
              ) : (
                <div className='p-3 sm:p-5'>
                  <Alert>
                    <AlertDescription>
                      {t(
                        'No payment methods available. Please contact administrator.'
                      )}
                    </AlertDescription>
                  </Alert>
                </div>
              )}
            </TitledCard>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
