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
import { Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Progress } from '@/components/ui/progress'
import type { ApiKey, TokenMetric } from '../types'

export function KeyQuotaCell(props: {
  apiKey: ApiKey
  metric?: TokenMetric
  unit: 'quota' | 'tokens'
  consumptionStatus?: string
}) {
  const { t } = useTranslation()
  const key = props.apiKey
  const today =
    props.unit === 'tokens'
      ? props.metric?.today_tokens
      : props.metric?.today_quota
  let todayLabel = '—'
  if (today != null)
    todayLabel =
      props.unit === 'tokens' ? today.toLocaleString() : formatQuota(today)
  const total = key.used_quota + key.remain_quota
  return (
    <div className='min-w-[142px] space-y-1 text-xs tabular-nums'>
      <div className='flex justify-between gap-3'>
        <span className='text-muted-foreground'>{t('Remaining')}</span>
        <span className='font-medium'>
          {key.unlimited_quota ? t('Unlimited') : formatQuota(key.remain_quota)}
        </span>
      </div>
      <div className='flex justify-between gap-3'>
        <span className='text-muted-foreground'>{t('Total used')}</span>
        <span>{formatQuota(key.used_quota)}</span>
      </div>
      <div
        className='flex justify-between gap-3'
        title={
          props.consumptionStatus === 'disabled'
            ? t('Consumption logging is disabled')
            : t(
                'Net usage after refunds · Asia/Shanghai · Updates every 30 seconds'
              )
        }
      >
        <span className='text-muted-foreground'>
          {t(props.unit === 'tokens' ? 'Today tokens' : 'Today net usage')}
        </span>
        <span className='text-primary font-medium'>{todayLabel}</span>
      </div>
      {!key.unlimited_quota && (
        <Progress
          value={
            total > 0
              ? Math.max(0, Math.min(100, (key.remain_quota / total) * 100))
              : 0
          }
          className='h-1'
        />
      )}
    </div>
  )
}
export function KeyActivityCell(props: { metric?: TokenMetric }) {
  const { t } = useTranslation()
  return (
    <div
      className='space-y-1.5 text-xs tabular-nums'
      title={t(
        'Active requests on this instance · RPM counts requests started in the last 60 seconds'
      )}
    >
      <div className='flex items-center gap-1.5'>
        <Activity className='size-3.5 text-emerald-600 dark:text-emerald-400' />
        <span className='font-semibold'>{props.metric?.active ?? '—'}</span>
        <span className='text-muted-foreground'>{t('Active')}</span>
      </div>
      <div className='text-muted-foreground'>
        {props.metric?.rpm ?? '—'} RPM
      </div>
    </div>
  )
}
function relativeTime(timestamp: number, language: string): string {
  const seconds = Math.round((timestamp * 1000 - Date.now()) / 1000)
  const abs = Math.abs(seconds)
  const formatter = new Intl.RelativeTimeFormat(language || 'zh', {
    numeric: 'auto',
  })
  if (abs < 60) return formatter.format(seconds, 'second')
  if (abs < 3600) return formatter.format(Math.round(seconds / 60), 'minute')
  if (abs < 86400) return formatter.format(Math.round(seconds / 3600), 'hour')
  return formatter.format(Math.round(seconds / 86400), 'day')
}
export function KeyTimeCell(props: { apiKey: ApiKey }) {
  const { t, i18n } = useTranslation()
  const key = props.apiKey
  return (
    <div className='text-muted-foreground space-y-1 text-xs'>
      <div title={formatTimestampToDate(key.created_time)}>
        {t('Created')} · {relativeTime(key.created_time, i18n.language)}
      </div>
      <div
        title={
          key.accessed_time
            ? formatTimestampToDate(key.accessed_time)
            : undefined
        }
      >
        {t('Last Used')} ·{' '}
        {key.accessed_time
          ? relativeTime(key.accessed_time, i18n.language)
          : '—'}
      </div>
      <div
        className={key.status === 3 ? 'text-destructive' : ''}
        title={
          key.expired_time === -1
            ? undefined
            : formatTimestampToDate(key.expired_time)
        }
      >
        {t('Expires')} ·{' '}
        {key.expired_time === -1
          ? t('Never')
          : relativeTime(key.expired_time, i18n.language)}
      </div>
    </div>
  )
}
