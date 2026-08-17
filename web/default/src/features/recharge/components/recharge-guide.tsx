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
import { useState } from 'react'
import {
  CircleHelp,
  ClipboardCopy,
  Search,
  ShoppingBag,
  TicketCheck,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const GUIDE_STORAGE_PREFIX = 'recharge-redemption-guide:v1'

type RechargeGuideProps = {
  userId?: number
  onGoToPurchase: () => void
  onGoToRedeem: () => void
}

function getStorageKey(userId: number) {
  return `${GUIDE_STORAGE_PREFIX}:${userId}`
}

function hasSeenGuide(userId: number) {
  try {
    return window.localStorage.getItem(getStorageKey(userId)) === 'seen'
  } catch {
    return false
  }
}

function markGuideAsSeen(userId?: number) {
  if (!userId) return

  try {
    window.localStorage.setItem(getStorageKey(userId), 'seen')
  } catch {
    // The permanent guide remains available when browser storage is blocked.
  }
}

export function RechargeGuide(props: RechargeGuideProps) {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(
    () => !!props.userId && !hasSeenGuide(props.userId)
  )

  const handleDialogOpenChange = (open: boolean) => {
    setDialogOpen(open)
    if (!open) markGuideAsSeen(props.userId)
  }

  const runDialogAction = (action: () => void) => {
    markGuideAsSeen(props.userId)
    setDialogOpen(false)
    window.requestAnimationFrame(action)
  }

  const steps = [
    {
      title: t('Buy a card code'),
      description: t(
        'Choose an amount in the store below and complete payment.'
      ),
      icon: ShoppingBag,
    },
    {
      title: t('Copy the card code'),
      description: t(
        'After payment, select "Get Card Code" and then "Copy All".'
      ),
      icon: ClipboardCopy,
    },
    {
      title: t('Redeem and add balance'),
      description: t('Return here, paste the card code, and select "Redeem".'),
      icon: TicketCheck,
    },
  ]

  return (
    <>
      <Alert className='border-amber-500/40 bg-amber-500/10 px-3 py-3 sm:px-4 sm:py-4'>
        <CircleHelp className='text-amber-700 dark:text-amber-400' />
        <AlertTitle className='text-sm sm:text-base'>
          {t('Payment completed? Redeem the card code to add balance')}
        </AlertTitle>
        <AlertDescription className='space-y-3 text-left'>
          <p>{t('Complete these three steps after purchasing a card code.')}</p>
          <ol className='grid gap-3 lg:grid-cols-3'>
            {steps.map((step, index) => {
              const StepIcon = step.icon

              return (
                <li key={step.title} className='flex min-w-0 gap-2.5'>
                  <span className='bg-background flex size-7 shrink-0 items-center justify-center rounded-full border text-xs font-semibold'>
                    {index + 1}
                  </span>
                  <div className='min-w-0'>
                    <div className='text-foreground flex items-center gap-1.5 font-medium'>
                      <StepIcon className='size-4 shrink-0' />
                      <span>{step.title}</span>
                    </div>
                    <p className='mt-1 text-xs leading-5'>{step.description}</p>
                  </div>
                </li>
              )
            })}
          </ol>
          <div className='flex flex-col gap-3 border-t border-amber-500/25 pt-3 sm:flex-row sm:items-center sm:justify-between'>
            <p className='text-xs leading-5 sm:max-w-2xl'>
              {t(
                'Closed the order page? Use "Order Lookup" with your contact information or order number to retrieve the card code.'
              )}
            </p>
            <div className='flex shrink-0 flex-col gap-2 sm:flex-row'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='w-full gap-2 sm:w-auto'
                onClick={() => setDialogOpen(true)}
              >
                <CircleHelp className='size-4' />
                {t('View detailed steps')}
              </Button>
              <Button
                type='button'
                size='sm'
                className='w-full gap-2 sm:w-auto'
                onClick={props.onGoToRedeem}
              >
                <TicketCheck className='size-4' />
                {t('I have a card code')}
              </Button>
            </div>
          </div>
        </AlertDescription>
      </Alert>

      <Dialog open={dialogOpen} onOpenChange={handleDialogOpenChange}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('How to use a purchased card code')}</DialogTitle>
            <DialogDescription>
              {t(
                'Payment is only the first step. Copy and redeem the card code to add the balance.'
              )}
            </DialogDescription>
          </DialogHeader>

          <ol className='divide-y rounded-lg border'>
            {steps.map((step, index) => {
              const StepIcon = step.icon

              return (
                <li key={step.title} className='flex gap-3 p-3'>
                  <span className='bg-muted flex size-8 shrink-0 items-center justify-center rounded-full font-semibold'>
                    {index + 1}
                  </span>
                  <div className='min-w-0'>
                    <div className='flex items-center gap-2 font-medium'>
                      <StepIcon className='size-4 shrink-0' />
                      <span>{step.title}</span>
                    </div>
                    <p className='text-muted-foreground mt-1 text-sm leading-6'>
                      {step.description}
                    </p>
                  </div>
                </li>
              )
            })}
          </ol>

          <Alert>
            <Search />
            <AlertTitle>{t('Closed the order page?')}</AlertTitle>
            <AlertDescription>
              {t(
                'Use "Order Lookup" with your contact information or order number to retrieve the card code.'
              )}
            </AlertDescription>
          </Alert>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => runDialogAction(props.onGoToRedeem)}
            >
              <TicketCheck className='size-4' />
              {t('Redeem my card code')}
            </Button>
            <Button
              type='button'
              onClick={() => runDialogAction(props.onGoToPurchase)}
            >
              <ShoppingBag className='size-4' />
              {t('Start purchasing')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
