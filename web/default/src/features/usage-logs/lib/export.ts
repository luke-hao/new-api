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
import {
  formatLogQuota,
  formatTimestampToDate,
  formatUseTime,
} from '@/lib/format'
import { buildLogDetailSegments } from '../components/columns/common-logs-columns'
import type { UsageLog } from '../data/schema'
import { formatModelName, parseLogOther } from './format'
import {
  getLogTypeConfig,
  isDisplayableLogType,
  isTimingLogType,
} from './utils'

type Translate = (key: string, options?: Record<string, unknown>) => string

const UTF8_BOM = '\uFEFF'
const FORMULA_PREFIX = /^[\t\r\n ]*[=+\-@]/

function csvCell(value: string | number | boolean | null | undefined): string {
  if (value === null || value === undefined) return '""'
  let text = String(value)
  if (typeof value === 'string' && FORMULA_PREFIX.test(text)) {
    text = `'${text}`
  }
  return `"${text.replace(/"/g, '""')}"`
}

function formatChannel(log: UsageLog): string {
  if (!log.channel) return ''
  return log.channel_name
    ? `${log.channel_name} #${log.channel}`
    : `#${log.channel}`
}

function formatModel(log: UsageLog): string {
  const model = formatModelName(log)
  return model.actualModel
    ? `${model.name} -> ${model.actualModel}`
    : model.name
}

function formatDetails(log: UsageLog, t: Translate): string {
  const details = buildLogDetailSegments(log, parseLogOther(log.other), t)
  if (details.length > 0) {
    return details.map((detail) => detail.text).join(' | ')
  }
  return log.content || ''
}

export function buildUsageLogsCsv(
  logs: UsageLog[],
  isAdmin: boolean,
  t: Translate
): string {
  const headers = [
    t('Time'),
    t('Type'),
    ...(isAdmin ? [t('Channel'), t('User')] : []),
    t('Token'),
    t('Group'),
    t('Model'),
    t('Reasoning Effort'),
    t('Total Duration'),
    t('First Response Time'),
    t('Request Mode'),
    t('Prompt Tokens'),
    t('Completion Tokens'),
    t('Cost'),
    t('Details'),
  ]

  const rows = logs.map((log) => {
    const other = parseLogOther(log.other)
    const displayable = isDisplayableLogType(log.type)
    const timing = isTimingLogType(log.type)
    const model = displayable ? formatModel(log) : ''
    const group = log.group || other?.group || ''
    const firstResponse =
      !timing || !log.is_stream
        ? ''
        : other?.frt && other.frt > 0
          ? formatUseTime(other.frt / 1000)
          : 'N/A'
    const cost = !displayable
      ? ''
      : other?.billing_source === 'subscription'
        ? t('Subscription')
        : formatLogQuota(log.quota)

    return [
      formatTimestampToDate(log.created_at),
      t(getLogTypeConfig(log.type).label),
      ...(isAdmin ? [formatChannel(log), log.username] : []),
      displayable ? log.token_name : '',
      displayable ? group : '',
      model,
      displayable ? other?.reasoning_effort?.trim() || '' : '',
      timing ? formatUseTime(log.use_time) : '',
      firstResponse,
      timing ? t(log.is_stream ? 'Stream' : 'Non-stream') : '',
      displayable ? log.prompt_tokens : '',
      displayable ? log.completion_tokens : '',
      cost,
      formatDetails(log, t),
    ]
  })

  return `${UTF8_BOM}${[headers, ...rows]
    .map((row) => row.map(csvCell).join(','))
    .join('\r\n')}`
}

export function createUsageLogsCsvFilename(now = new Date()): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `usage-logs-${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(
    now.getDate()
  )}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}.csv`
}

export function downloadUsageLogsCsv(content: string, filename: string) {
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}
