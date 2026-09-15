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
import zh from '@/i18n/locales/zh.json'
// @ts-expect-error -- bun:test is provided by Bun and excluded from app types.
import { expect, test } from 'bun:test'
import type { LogOtherData } from '../types'
import { getReasoningDisplay } from './format'

const t = (key: string, options?: Record<string, unknown>) =>
  key.replace('{{tokens}}', String(options?.tokens ?? ''))

test.each([
  [{ reasoning_effort: 'xhigh' }, 'xhigh'],
  [
    { reasoning_effort: 'future-effort', reasoning_status: 'specified' },
    'future-effort',
  ],
  [{ reasoning_effort: 'none' }, 'Reasoning disabled'],
  [
    { reasoning_effort: 'high', reasoning_status: 'disabled' },
    'Reasoning disabled',
  ],
  [
    { reasoning_status: 'enabled', thinking_budget_tokens: 8192 },
    'Thinking budget: 8,192 tokens',
  ],
  [
    { reasoning_status: 'disabled', thinking_budget_tokens: 0 },
    'Reasoning disabled',
  ],
  [
    { reasoning_status: 'automatic', thinking_budget_tokens: -1 },
    'Reasoning automatic',
  ],
  [{ reasoning_status: 'automatic' }, 'Reasoning automatic'],
  [{ reasoning_status: 'enabled' }, 'Reasoning enabled'],
  [{ reasoning_status: 'unspecified' }, 'Reasoning unspecified'],
  [{ reasoning_status: 'unknown' }, 'Reasoning unknown'],
  [{ reasoning_status: 'not_applicable' }, 'Reasoning not applicable'],
  [{ request_path: '/v1/images/edits' }, 'Reasoning not applicable'],
  [{ request_path: '/pg/videos' }, 'Reasoning not applicable'],
  [
    { request_path: '/v1beta/models/example:embedContent' },
    'Reasoning not applicable',
  ],
  [
    { request_path: '/v1/responses', model: 'example-high' },
    'Reasoning not recorded',
  ],
  [{}, 'Reasoning not recorded'],
  [null, 'Reasoning not recorded'],
] as const)('reasoning display %j', (other: unknown, expected: string) => {
  expect(getReasoningDisplay(other as LogOtherData | null, t).label).toBe(
    expected
  )
})

test('malformed legacy effort does not crash rendering', () => {
  const other = { reasoning_effort: 42 } as unknown as LogOtherData
  expect(getReasoningDisplay(other, t).label).toBe('Reasoning not recorded')
})

test('Chinese reasoning labels are available in the loaded namespace', () => {
  const translate = (key: string, options?: Record<string, unknown>) => {
    const text = (zh.translation as Record<string, string>)[key] ?? key
    return text.replace('{{tokens}}', String(options?.tokens ?? ''))
  }
  expect(
    getReasoningDisplay({ reasoning_status: 'unspecified' }, translate).label
  ).toBe('\u672a\u6307\u5b9a')
  expect(
    getReasoningDisplay({ reasoning_status: 'automatic' }, translate).label
  ).toBe('\u81ea\u52a8')
  expect(
    getReasoningDisplay(
      { reasoning_status: 'enabled', thinking_budget_tokens: 8192 },
      translate
    ).label
  ).toBe('\u9884\u7b97 8,192 tokens')
})
