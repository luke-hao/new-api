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
import { ChevronLeft, ChevronRight, Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber, formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/dialog'
import { useAffiliateRebates } from '../../hooks'
import { formatTimestamp } from '../../lib/billing'

interface AffiliateRebateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AffiliateRebateDialog({
  open,
  onOpenChange,
}: AffiliateRebateDialogProps) {
  const { t } = useTranslation()
  const { records, total, page, pageSize, loading, error, setPage, refetch } =
    useAffiliateRebates(open)
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Referral Rebate Details')}
      description={t(
        'Rebates are calculated from successful referral payments or redeemed quota and added to your referral reward balance.'
      )}
      contentClassName='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-3xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      <ScrollArea className='max-h-[min(58vh,560px)] pr-3 sm:pr-4'>
        {loading ? (
          <div className='space-y-3'>
            {Array.from({ length: 4 }).map((_, index) => (
              <Skeleton key={index} className='h-28 w-full rounded-lg' />
            ))}
          </div>
        ) : error ? (
          <div className='flex min-h-40 flex-col items-center justify-center gap-3 text-center'>
            <p className='text-muted-foreground text-sm'>
              {t('Failed to load rebate details')}
            </p>
            <Button variant='outline' size='sm' onClick={() => void refetch()}>
              {t('Retry')}
            </Button>
          </div>
        ) : records.length === 0 ? (
          <div className='text-muted-foreground flex min-h-40 flex-col items-center justify-center text-center'>
            <Gift className='mb-3 size-8 opacity-60' />
            <p className='text-sm font-medium'>{t('No rebate records yet')}</p>
            <p className='mt-1 text-xs'>
              {t(
                'New rewards will appear after a referral completes a payment.'
              )}
            </p>
          </div>
        ) : (
          <div className='space-y-3'>
            {records.map((record) => (
              <div key={record.id} className='rounded-lg border p-3 sm:p-4'>
                <div className='flex items-start justify-between gap-3'>
                  <div>
                    <p className='text-sm font-semibold'>
                      {record.invitee || t('Deleted user')}
                    </p>
                    <p className='text-muted-foreground mt-0.5 text-xs'>
                      {formatTimestamp(record.created_at)}
                    </p>
                  </div>
                  <div className='text-right'>
                    <p className='text-muted-foreground text-xs'>
                      {t('Rebate')}
                    </p>
                    <p className='text-sm font-semibold text-emerald-600'>
                      +{formatQuota(record.rebate_quota)}
                    </p>
                  </div>
                </div>
                <div className='mt-3 grid grid-cols-2 gap-3 border-t pt-3'>
                  <div>
                    <Label className='text-muted-foreground text-xs'>
                      {record.source_type === 'redemption'
                        ? t('Redeemed Quota')
                        : t('Referral Payment')}
                    </Label>
                    <p className='mt-1 text-sm font-medium tabular-nums'>
                      {record.source_type === 'redemption'
                        ? formatQuota(record.source_quota)
                        : formatNumber(record.paid_money)}
                    </p>
                  </div>
                  <div>
                    <Label className='text-muted-foreground text-xs'>
                      {t('Rebate Rate')}
                    </Label>
                    <p className='mt-1 text-sm font-medium tabular-nums'>
                      {formatNumber(record.rebate_percent)}%
                    </p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </ScrollArea>

      {!loading && !error && total > pageSize ? (
        <div className='flex items-center justify-between border-t pt-3'>
          <p className='text-muted-foreground text-xs'>
            {t('{{count}} rebate records', { count: total })}
          </p>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='icon'
              className='size-8'
              onClick={() => setPage(Math.max(1, page - 1))}
              disabled={page <= 1}
              aria-label={t('Previous page')}
            >
              <ChevronLeft className='size-4' />
            </Button>
            <span className='min-w-14 text-center text-xs tabular-nums'>
              {page} / {totalPages}
            </span>
            <Button
              variant='outline'
              size='icon'
              className='size-8'
              onClick={() => setPage(Math.min(totalPages, page + 1))}
              disabled={page >= totalPages}
              aria-label={t('Next page')}
            >
              <ChevronRight className='size-4' />
            </Button>
          </div>
        </div>
      ) : null}
    </Dialog>
  )
}
