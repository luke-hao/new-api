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
import { z } from 'zod'

const ruleSchema = z.object({
  enabled: z.boolean(),
  interval_minutes: z.number().int().min(1).max(10080),
  successes_required: z.number().int().min(1).max(10),
})
export const recoverySchema = z.object({
  enabled: z.boolean(),
  rules: z.object({
    balance: ruleSchema,
    rate_limit: ruleSchema,
    timeout: ruleSchema,
    authentication: ruleSchema,
    other: ruleSchema,
  }),
})
export type RecoveryPolicy = z.infer<typeof recoverySchema>
export const defaultRecoveryPolicy: RecoveryPolicy = {
  enabled: false,
  rules: {
    balance: { enabled: true, interval_minutes: 30, successes_required: 1 },
    rate_limit: { enabled: true, interval_minutes: 2, successes_required: 2 },
    timeout: { enabled: true, interval_minutes: 5, successes_required: 2 },
    authentication: {
      enabled: false,
      interval_minutes: 60,
      successes_required: 1,
    },
    other: { enabled: false, interval_minutes: 10, successes_required: 2 },
  },
}
export function parseRecoveryPolicy(raw: string): RecoveryPolicy {
  try {
    return recoverySchema.parse(JSON.parse(raw))
  } catch {
    return structuredClone(defaultRecoveryPolicy)
  }
}
export const recoveryReasonLabels = {
  balance: 'Insufficient upstream balance',
  rate_limit: 'Rate limited',
  timeout: 'Timeout or slow response',
  authentication: 'Authentication or permission error',
  other: 'Other channel errors',
} as const
