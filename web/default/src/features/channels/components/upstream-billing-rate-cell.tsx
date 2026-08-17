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
import { useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { probeChannelUpstreamBilling } from '../api'
import {
  channelsQueryKeys,
  formatUpstreamBillingRate,
  isTagAggregateRow,
  parseUpstreamBillingProbeSnapshot,
} from '../lib'
import type {
  Channel,
  UpstreamBillingKeyResult,
  UpstreamBillingProbeSnapshot,
} from '../types'

type DisplayState = {
  label: string
  statusLabel: string
  variant: StatusVariant
  hasHistoricalRate: boolean
}

function formatRateRange(
  snapshot: UpstreamBillingProbeSnapshot
): string | null {
  const minimum = snapshot.effective_rate_min
  const maximum = snapshot.effective_rate_max
  if (minimum === undefined || maximum === undefined) return null
  if (snapshot.consistent || minimum === maximum) {
    return `${formatUpstreamBillingRate(minimum)}×`
  }
  return `${formatUpstreamBillingRate(minimum)}–${formatUpstreamBillingRate(maximum)}×`
}

function getDisplayState(
  snapshot: UpstreamBillingProbeSnapshot | null,
  t: (key: string) => string
): DisplayState {
  if (!snapshot) {
    return {
      label: t('Not probed'),
      statusLabel: t('Not probed'),
      variant: 'neutral',
      hasHistoricalRate: false,
    }
  }

  const rate = formatRateRange(snapshot)
  if (snapshot.status === 'ok') {
    if (!snapshot.consistent) {
      return {
        label: rate ?? t('Rate mismatch'),
        statusLabel: t('Rate mismatch'),
        variant: 'warning',
        hasHistoricalRate: false,
      }
    }
    return {
      label: rate ?? t('Available'),
      statusLabel: t('Available'),
      variant: 'success',
      hasHistoricalRate: false,
    }
  }
  if (snapshot.status === 'partial') {
    return {
      label: rate ? `${t('Partial')} · ${rate}` : t('Partial'),
      statusLabel: t('Partial success'),
      variant: 'warning',
      hasHistoricalRate: false,
    }
  }
  if (snapshot.status === 'unsupported') {
    return {
      label: t('Not supported'),
      statusLabel: t('Not supported'),
      variant: 'neutral',
      hasHistoricalRate: false,
    }
  }
  return {
    label: rate ? `${t('Last known')} · ${rate}` : t('Failed'),
    statusLabel: t('Failed'),
    variant: 'danger',
    hasHistoricalRate: Boolean(rate),
  }
}

function formatProbeTime(timestamp?: number): string {
  return timestamp ? formatTimestampToDate(timestamp) : '—'
}

function formatObservedTime(value?: string): string {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

function KeyProbeDetails({ result }: { result: UpstreamBillingKeyResult }) {
  const { t } = useTranslation()
  const billing = result.billing
  const status = getDisplayState(
    result.status === 'ok'
      ? {
          status: 'ok',
          attempted_at: result.attempted_at,
          last_success_at: result.last_success_at,
          total_keys: 1,
          success_count: 1,
          unsupported_count: 0,
          failed_count: 0,
          consistent: true,
          effective_rate_min: billing?.effective_rate_multiplier,
          effective_rate_max: billing?.effective_rate_multiplier,
          key_results: [result],
        }
      : null,
    t
  )
  const resultStatus =
    result.status === 'unsupported'
      ? { label: t('Not supported'), variant: 'neutral' as const }
      : result.status === 'failed'
        ? { label: t('Failed'), variant: 'danger' as const }
        : { label: status.statusLabel, variant: 'success' as const }

  return (
    <section className='border-border/70 space-y-3 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0'>
          <div className='font-medium'>
            {t('Key')} #{result.key_index + 1}
          </div>
          <div className='text-muted-foreground truncate font-mono text-xs'>
            {result.key_mask || '—'} · {result.key_fingerprint}
          </div>
        </div>
        <StatusBadge
          label={resultStatus.label}
          variant={resultStatus.variant}
          copyable={false}
        />
      </div>

      {result.status !== 'ok' && (
        <div className='bg-muted/50 grid grid-cols-2 gap-2 rounded-md p-2 text-xs'>
          <div>
            <span className='text-muted-foreground'>{t('Error code')}: </span>
            <span className='font-mono'>{result.error_code || '—'}</span>
          </div>
          <div>
            <span className='text-muted-foreground'>{t('HTTP status')}: </span>
            <span className='font-mono'>{result.http_status || '—'}</span>
          </div>
        </div>
      )}

      {billing ? (
        <div className='grid grid-cols-2 gap-x-4 gap-y-2 text-xs sm:grid-cols-3'>
          <RateDetail
            label={t('Default multiplier')}
            value={`${formatUpstreamBillingRate(billing.group_rate_multiplier)}×`}
          />
          <RateDetail
            label={t('User multiplier')}
            value={
              billing.user_rate_multiplier === undefined
                ? '—'
                : `${formatUpstreamBillingRate(billing.user_rate_multiplier)}×`
            }
          />
          <RateDetail
            label={t('Resolved multiplier')}
            value={`${formatUpstreamBillingRate(billing.resolved_rate_multiplier)}×`}
          />
          <RateDetail
            label={t('Peak period')}
            value={
              billing.peak_rate_enabled
                ? `${billing.peak_start ?? '—'}–${billing.peak_end ?? '—'} (${billing.timezone ?? '—'})`
                : t('Disabled')
            }
          />
          <RateDetail
            label={t('Peak multiplier')}
            value={
              billing.peak_rate_enabled &&
              billing.peak_rate_multiplier !== undefined
                ? `${formatUpstreamBillingRate(billing.peak_rate_multiplier)}× (${t('applied')} ${formatUpstreamBillingRate(billing.applied_peak_multiplier ?? 1)}×)`
                : '—'
            }
          />
          <RateDetail
            label={t('Effective multiplier')}
            value={`${formatUpstreamBillingRate(billing.effective_rate_multiplier)}×`}
            emphasized
          />
          <RateDetail
            label={t('Upstream observed at')}
            value={formatObservedTime(billing.observed_at)}
          />
          <RateDetail
            label={t('Last successful probe')}
            value={formatProbeTime(result.last_success_at)}
          />
          {result.status !== 'ok' && (
            <RateDetail
              label={t('Historical value')}
              value={t('Preserved from the last successful probe')}
            />
          )}
        </div>
      ) : (
        <p className='text-muted-foreground text-xs'>
          {t('No successful multiplier data is available for this key.')}
        </p>
      )}
    </section>
  )
}

function RateDetail({
  label,
  value,
  emphasized = false,
}: {
  label: string
  value: string
  emphasized?: boolean
}) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground'>{label}</div>
      <div
        className={emphasized ? 'text-primary font-semibold' : 'font-medium'}
      >
        {value}
      </div>
    </div>
  )
}

export function UpstreamBillingRateHeader() {
  const { t } = useTranslation()
  return (
    <Tooltip>
      <TooltipTrigger
        render={<span className='inline-flex items-center gap-1' />}
      >
        {t('Upstream multiplier')}
        <AlertTriangle className='size-3.5 text-amber-500' />
      </TooltipTrigger>
      <TooltipContent className='max-w-xs'>
        {t(
          'The multiplier is declared by the upstream and cannot replace verification against actual billing.'
        )}
      </TooltipContent>
    </Tooltip>
  )
}

export function UpstreamBillingRateCell({ channel }: { channel: Channel }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const parsedSnapshot = parseUpstreamBillingProbeSnapshot(channel.other_info)
  const [probeSnapshot, setProbeSnapshot] = useState<
    UpstreamBillingProbeSnapshot | undefined
  >()
  const [isProbing, setIsProbing] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const snapshot = probeSnapshot ?? parsedSnapshot

  if (isTagAggregateRow(channel)) {
    return <span className='text-muted-foreground text-xs'>—</span>
  }

  const display = getDisplayState(snapshot, t)
  const probe = async () => {
    if (isProbing) return
    setIsProbing(true)
    try {
      const response = await probeChannelUpstreamBilling(channel.id)
      if (!response.success || !response.data) {
        toast.error(
          response.message || t('Failed to probe upstream multiplier')
        )
        return
      }
      setProbeSnapshot(response.data)
      if (response.data.status === 'ok' && response.data.consistent) {
        toast.success(
          response.data.name_updated
            ? t('Upstream multiplier and channel name synchronized')
            : t('Upstream multiplier sync completed')
        )
      } else {
        toast.warning(
          t('Upstream multiplier probe completed with status: {{status}}', {
            status: getDisplayState(response.data, t).statusLabel,
          })
        )
      }
      await queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.lists(),
      })
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to probe upstream multiplier')
      )
    } finally {
      setIsProbing(false)
    }
  }

  return (
    <>
      <div className='-ml-1.5 flex items-center gap-1'>
        <Tooltip>
          <TooltipTrigger
            render={
              <StatusBadge
                label={display.label}
                variant={display.variant}
                copyable={false}
                className={snapshot ? 'cursor-pointer' : undefined}
                onClick={() => snapshot && setDetailsOpen(true)}
              />
            }
          />
          <TooltipContent className='max-w-xs'>
            <div>{display.statusLabel}</div>
            {display.hasHistoricalRate && (
              <div>{t('The displayed rate is the last successful value.')}</div>
            )}
            {snapshot && <div>{t('Click to view per-key details')}</div>}
          </TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='ghost'
                size='icon-sm'
                className='size-7'
                disabled={isProbing}
                onClick={(event) => {
                  event.stopPropagation()
                  void probe()
                }}
                aria-label={t('Sync upstream multiplier and channel name')}
              />
            }
          >
            <RefreshCw className={isProbing ? 'animate-spin' : undefined} />
          </TooltipTrigger>
          <TooltipContent>
            {t('Sync upstream multiplier and channel name')}
          </TooltipContent>
        </Tooltip>
      </div>

      <Sheet open={detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>{t('Upstream multiplier details')}</SheetTitle>
            <SheetDescription>
              {channel.name} · #{channel.id}
            </SheetDescription>
          </SheetHeader>
          {snapshot && (
            <div className='min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-4 sm:px-6'>
              <div className='border-warning/40 bg-warning/5 flex gap-2 rounded-lg border p-3 text-xs'>
                <AlertTriangle className='text-warning mt-0.5 size-4 shrink-0' />
                <span>
                  {t(
                    'The multiplier is declared by the upstream and cannot replace verification against actual billing.'
                  )}
                </span>
              </div>
              <div className='bg-muted/40 grid grid-cols-2 gap-3 rounded-lg p-3 text-xs sm:grid-cols-3'>
                <RateDetail label={t('Status')} value={display.statusLabel} />
                <RateDetail
                  label={t('Keys')}
                  value={`${snapshot.success_count}/${snapshot.total_keys} ${t('successful')}`}
                />
                <RateDetail
                  label={t('Effective multiplier')}
                  value={formatRateRange(snapshot) ?? '—'}
                  emphasized
                />
                <RateDetail
                  label={t('Attempted at')}
                  value={formatProbeTime(snapshot.attempted_at)}
                />
                <RateDetail
                  label={t('Last successful probe')}
                  value={formatProbeTime(snapshot.last_success_at)}
                />
                <RateDetail
                  label={t('Result summary')}
                  value={`${snapshot.success_count} ${t('successful')} / ${snapshot.unsupported_count} ${t('unsupported')} / ${snapshot.failed_count} ${t('failed')}`}
                />
              </div>
              <div className='space-y-3'>
                {snapshot.key_results.map((result) => (
                  <KeyProbeDetails
                    key={`${result.key_index}-${result.key_fingerprint}`}
                    result={result}
                  />
                ))}
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </>
  )
}
