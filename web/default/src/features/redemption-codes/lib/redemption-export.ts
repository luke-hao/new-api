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
import { formatQuota } from '@/lib/format'
import { type Redemption } from '../types'

const PANCAKE_CARD_SEPARATOR = '------------'

function downloadTextFile(filename: string, content: string) {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

function sanitizeFilenamePart(value: string) {
  return value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, '-')
    .slice(0, 48)
}

export function formatRedemptionForPancake(redemption: Redemption) {
  return `${redemption.key}${PANCAKE_CARD_SEPARATOR}${formatQuota(redemption.quota)}`
}

export function formatCreatedRedemptionsForPancake(
  keys: string[],
  quota: number
) {
  const amount = formatQuota(quota)
  return keys
    .map((key) => `${key}${PANCAKE_CARD_SEPARATOR}${amount}`)
    .join('\n')
}

export function downloadRedemptionsForPancake(
  redemptions: Redemption[],
  filename = 'redemption-codes-pancake.txt'
) {
  const content = redemptions.map(formatRedemptionForPancake).join('\n')
  downloadTextFile(filename, content)
}

export function downloadCreatedRedemptionsForPancake(
  keys: string[],
  quota: number,
  name: string
) {
  const safeName = sanitizeFilenamePart(name) || 'redemption-codes'
  const content = formatCreatedRedemptionsForPancake(keys, quota)
  downloadTextFile(`${safeName}-pancake.txt`, content)
}
