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
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import type { Channel, ChannelGroupRoutingUpdateParams } from '../types'

let fixtures: Channel[] = []
let calls: ChannelGroupRoutingUpdateParams[] = []
let serverSkip = 0
let fail = false
mock.module('../api', () => ({
  getChannels: async () => ({
    success: true,
    data: { items: fixtures, total: fixtures.length, page_size: 100 },
  }),
  updateChannelGroupRouting: async (
    request: ChannelGroupRoutingUpdateParams
  ) => {
    calls.push(request)
    if (fail) return { success: false, message: 'fixture failure' }
    return {
      success: true,
      data: {
        updated: request.updates.length - serverSkip,
        skipped_locked: serverSkip,
      },
    }
  },
}))
const { rankGroupChannelsByLowestPrice, extractChannelPriceRatio } =
  await import('./group-priority-actions')
function channel(id: number, name: string, priority: number, locked = false) {
  return {
    id,
    name,
    priority,
    effective_priority: priority,
    priority_locked: locked,
  } as Channel
}
beforeEach(() => {
  fixtures = []
  calls = []
  serverSkip = 0
  fail = false
})
describe('fixed group priorities', () => {
  test('keeps existing price parsing', () => {
    expect(extractChannelPriceRatio('pool 0.12')).toBe(0.12)
    expect(extractChannelPriceRatio('pool')).toBeNull()
  })
  test('excludes fixed channels from ranking and writes', async () => {
    fixtures = [
      channel(1, 'fixed 0.01', 7, true),
      channel(2, 'slow 0.2', 0),
      channel(3, 'fast 0.1', 0),
    ]
    const result = await rankGroupChannelsByLowestPrice('group-a')
    expect(result.skippedLocked).toBe(1)
    expect(result.participating).toBe(2)
    expect(calls[0].mode).toBe('rerank')
    expect(calls[0].updates).toEqual([
      { channel_id: 2, priority: 1 },
      { channel_id: 3, priority: 2 },
    ])
  })
  test('all fixed produces no write and no price-zero reset', async () => {
    fixtures = [channel(1, 'fixed', 7, true), channel(2, 'fixed 0.1', 0, true)]
    const result = await rankGroupChannelsByLowestPrice('group-a')
    expect(calls).toHaveLength(0)
    expect(result.updated).toBe(0)
    expect(result.participating).toBe(0)
    expect(result.skippedLocked).toBe(2)
  })
  test('counts locks changed after loading using the server response', async () => {
    fixtures = [channel(1, 'a 0.1', 7), channel(2, 'b 0.2', 7)]
    serverSkip = 1
    const result = await rankGroupChannelsByLowestPrice('group-a')
    expect(result.updated).toBe(1)
    expect(result.skippedLocked).toBe(1)
    expect(result.participating).toBe(1)
  })
  test('unpriced unlocked channels keep existing reset-to-zero behavior', async () => {
    fixtures = [channel(1, 'fixed', 7, true), channel(2, 'unpriced', 7)]
    const result = await rankGroupChannelsByLowestPrice('group-a')
    expect(calls[0].updates).toEqual([{ channel_id: 2, priority: 0 }])
    expect(result.unpriced).toBe(1)
  })
  test('unchanged priorities do not send redundant updates', async () => {
    fixtures = [channel(1, 'fixed', -5, true), channel(2, 'a 0.1', 1)]
    const result = await rankGroupChannelsByLowestPrice('group-a')
    expect(calls).toHaveLength(0)
    expect(result.unchanged).toBe(1)
    expect(result.skippedLocked).toBe(1)
  })
  test('reports update failures without counting fixed channels as failed', async () => {
    fixtures = [channel(1, 'fixed', 7, true), channel(2, 'a 0.1', 7)]
    fail = true
    const result = await rankGroupChannelsByLowestPrice('group-a')
    expect(result.updated).toBe(0)
    expect(result.failedUpdates).toBe(1)
    expect(result.skippedLocked).toBe(1)
  })
})
