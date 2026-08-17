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
// @ts-expect-error -- bun:test is provided by Bun and excluded from app types.
import { describe, expect, test } from 'bun:test'
import type { UsageLog } from '../data/schema'
import { buildUsageLogsCsv, createUsageLogsCsvFilename } from './export'
import { getReasoningEffortVariant } from './format'

const t = (key: string) => key

function makeLog(overrides: Partial<UsageLog> = {}): UsageLog {
  return {
    id: 1,
    user_id: 10,
    created_at: 1_700_000_000,
    type: 2,
    content: 'details',
    username: 'alice',
    token_name: 'main',
    model_name: 'model-a',
    quota: 500_000,
    prompt_tokens: 12,
    completion_tokens: 34,
    use_time: 1.5,
    is_stream: true,
    channel: 7,
    channel_name: 'Primary',
    token_id: 3,
    group: 'default',
    ip: '',
    other: '{"frt":250}',
    request_id: '',
    upstream_request_id: '',
    ...overrides,
  }
}

describe('buildUsageLogsCsv', () => {
  test('adds UTF-8 BOM and includes admin-only columns', () => {
    const csv = buildUsageLogsCsv([makeLog()], true, t)
    expect(csv.startsWith('\uFEFF')).toBe(true)
    expect(csv).toContain('"Channel","User"')
    expect(csv).toContain('"Primary #7","alice"')
    expect(csv).toContain('"Model","Reasoning Effort","Total Duration"')
    expect(csv).toContain('"1.5s","0.3s","Stream"')
  })

  test('omits admin-only columns for self exports', () => {
    const csv = buildUsageLogsCsv([makeLog()], false, t)
    expect(csv).not.toContain('"Channel"')
    expect(csv).not.toContain('"User"')
    expect(csv).not.toContain('"Primary #7"')
    expect(csv).not.toContain('"alice"')
  })

  test('exports reasoning effort and leaves missing values blank', () => {
    const csv = buildUsageLogsCsv(
      [
        makeLog({ other: '{"frt":250,"reasoning_effort":"xhigh"}' }),
        makeLog({ id: 2, other: '{"frt":250}' }),
      ],
      false,
      t
    )

    expect(csv).toContain('"Model","Reasoning Effort","Total Duration"')
    expect(csv).toContain('"model-a","xhigh","1.5s"')
    expect(csv).toContain('"model-a","","1.5s"')
  })

  test('escapes quotes and newlines and neutralizes spreadsheet formulas', () => {
    const csv = buildUsageLogsCsv(
      [
        makeLog({
          token_name: '=SUM(1,2)',
          content: 'line "one"\n@next',
        }),
      ],
      false,
      t
    )
    expect(csv).toContain('"\'=SUM(1,2)"')
    expect(csv).toContain('"line ""one""\n@next"')
  })
})

test('getReasoningEffortVariant maps known and unknown strengths', () => {
  expect(getReasoningEffortVariant('none')).toBe('green')
  expect(getReasoningEffortVariant('minimal')).toBe('green')
  expect(getReasoningEffortVariant('low')).toBe('green')
  expect(getReasoningEffortVariant('medium')).toBe('yellow')
  expect(getReasoningEffortVariant('high')).toBe('orange')
  expect(getReasoningEffortVariant('xhigh')).toBe('orange')
  expect(getReasoningEffortVariant('max')).toBe('orange')
  expect(getReasoningEffortVariant('future-effort')).toBe('neutral')
  expect(getReasoningEffortVariant()).toBe('neutral')
})

test('createUsageLogsCsvFilename uses local timestamp components', () => {
  const date = new Date(2026, 7, 6, 1, 2, 3)
  expect(createUsageLogsCsvFilename(date)).toBe(
    'usage-logs-20260806-010203.csv'
  )
})
