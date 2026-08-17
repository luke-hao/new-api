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
import { getChannels, updateChannelGroupRouting } from '../api'
import type { Channel } from '../types'

const GROUP_CHANNELS_PAGE_SIZE = 100
const TRAILING_PRICE_PATTERN = /(\d+(?:\.\d+)?)\s*$/

type PriorityUpdate = {
  id: number
  priority: number
}

export type GroupPriorityUpdateSummary = {
  total: number
  updated: number
  unchanged: number
  failedUpdates: number
}

export type PricePriorityResult = GroupPriorityUpdateSummary & {
  priced: number
  unpriced: number
}

export function extractChannelPriceRatio(name: string): number | null {
  const match = name.trim().match(TRAILING_PRICE_PATTERN)
  if (!match) return null

  const value = Number(match[1])
  return Number.isFinite(value) ? value : null
}

export async function fetchAllChannelsForGroup(
  group: string
): Promise<Channel[]> {
  const normalizedGroup = group.trim()
  if (!normalizedGroup) return []

  const channels: Channel[] = []
  let page = 1

  for (;;) {
    const response = await getChannels({
      group: normalizedGroup,
      p: page,
      page_size: GROUP_CHANNELS_PAGE_SIZE,
      tag_mode: false,
      id_sort: false,
    })

    if (!response.success) {
      throw new Error(response.message || 'Failed to load group channels')
    }

    const data = response.data
    const items = data?.items ?? []
    channels.push(...items)

    const total = data?.total ?? channels.length
    const pageSize = data?.page_size ?? GROUP_CHANNELS_PAGE_SIZE
    if (
      items.length === 0 ||
      channels.length >= total ||
      items.length < pageSize
    ) {
      break
    }

    page += 1
  }

  return channels
}

async function applyPriorityUpdates(
  group: string,
  channels: Channel[],
  priorities: Map<number, number>
): Promise<GroupPriorityUpdateSummary> {
  const updates = channels.reduce<PriorityUpdate[]>((items, channel) => {
    const priority = priorities.get(channel.id)
    if (typeof priority !== 'number') return items

    if ((channel.effective_priority ?? channel.priority ?? 0) !== priority) {
      items.push({ id: channel.id, priority })
    }

    return items
  }, [])

  let failedUpdates = 0
  if (updates.length > 0) {
    try {
      const response = await updateChannelGroupRouting({
        group,
        updates: updates.map((update) => ({
          channel_id: update.id,
          priority: update.priority,
        })),
      })
      if (!response.success) failedUpdates = updates.length
    } catch {
      failedUpdates = updates.length
    }
  }

  return {
    total: channels.length,
    updated: updates.length - failedUpdates,
    unchanged: channels.length - updates.length,
    failedUpdates,
  }
}

export async function rankGroupChannelsByLowestPrice(
  group: string
): Promise<PricePriorityResult> {
  const channels = await fetchAllChannelsForGroup(group)
  const priorities = new Map<number, number>()

  const pricedChannels = channels
    .map((channel) => ({
      channel,
      price: extractChannelPriceRatio(channel.name),
    }))
    .filter(
      (item): item is { channel: Channel; price: number } => item.price !== null
    )
    .sort((a, b) => a.price - b.price || a.channel.id - b.channel.id)

  channels.forEach((channel) => priorities.set(channel.id, 0))
  pricedChannels.forEach((item, index) => {
    priorities.set(item.channel.id, pricedChannels.length - index)
  })

  const summary = await applyPriorityUpdates(group, channels, priorities)
  return {
    ...summary,
    priced: pricedChannels.length,
    unpriced: channels.length - pricedChannels.length,
  }
}
