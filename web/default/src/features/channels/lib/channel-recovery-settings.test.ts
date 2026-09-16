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
import { channelSchema } from '../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from './channel-form'

describe('channel thinking recovery opt-in', () => {
  test('new and legacy channels stay disabled', () => {
    expect(CHANNEL_FORM_DEFAULT_VALUES.claude_thinking_recovery_enabled).toBe(
      false
    )
    const channel = channelSchema.parse({
      id: 901,
      key: '',
      status: 1,
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      name: 'fixture',
      type: 14,
      models: 'claude-test',
      group: 'default',
      setting: '{}',
    })
    const form = transformChannelToFormDefaults(channel)
    expect(form.claude_thinking_recovery_enabled).toBe(false)
    expect(
      JSON.parse(transformFormDataToUpdatePayload(form, channel.id).setting!)
        .claude_thinking_recovery_enabled
    ).toBe(false)
  })
  test('saving and reopening preserve both explicit choices', () => {
    for (const enabled of [true, false]) {
      const form = {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'fixture',
        type: 14,
        key: 'fixture-key',
        models: 'claude-test',
        group: ['default'],
        claude_thinking_recovery_enabled: enabled,
      }
      const created = transformFormDataToCreatePayload(form).channel
      expect(
        JSON.parse(created.setting!).claude_thinking_recovery_enabled
      ).toBe(enabled)
      const channel = channelSchema.parse({
        ...created,
        id: 901,
        created_time: 0,
        test_time: 0,
        response_time: 0,
        balance_updated_time: 0,
        other: '',
        remark: '',
      })
      const reopened = transformChannelToFormDefaults(channel)
      expect(reopened.claude_thinking_recovery_enabled).toBe(enabled)
      const updated = transformFormDataToUpdatePayload(reopened, channel.id)
      expect(
        JSON.parse(updated.setting!).claude_thinking_recovery_enabled
      ).toBe(enabled)
    }
  })
})
