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
import type { Modality, ModelCapability, PricingModel } from '../types'
import { VERIFIED_MODEL_METADATA } from './official-model-metadata'

export type ModelMetadata = {
  context_length: number
  max_output_tokens: number
  context_display: string
  max_output_display: string
  knowledge_cutoff: string
  release_date: string
  parameter_count: string
  input_modalities: Modality[]
  output_modalities: Modality[]
  capabilities: ModelCapability[]
  sources: string[]
  checked_at: string
}

function positiveNumber(value: number | undefined): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? value
    : 0
}

function validDate(value: string | undefined): string {
  if (!value || !/^\d{4}-\d{2}(?:-\d{2})?$/.test(value)) return ''
  const [year, month, day = 1] = value.split('-').map(Number)
  const date = new Date(Date.UTC(year, month - 1, day))
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day
  )
    return ''
  return value
}

/** Resolve explicit metadata or an exact, sourced model entry. Missing facts stay unknown. */
export function inferModelMetadata(model: PricingModel): ModelMetadata {
  const official =
    VERIFIED_MODEL_METADATA[(model.model_name || '').trim().toLowerCase()]
  return {
    context_length: positiveNumber(
      model.context_length ?? official?.context_length
    ),
    max_output_tokens: positiveNumber(
      model.max_output_tokens ?? official?.max_output_tokens
    ),
    context_display:
      model.context_length == null ? (official?.context_display ?? '') : '',
    max_output_display:
      model.max_output_tokens == null
        ? (official?.max_output_display ?? '')
        : '',
    knowledge_cutoff: validDate(
      model.knowledge_cutoff ?? official?.knowledge_cutoff
    ),
    release_date: validDate(model.release_date ?? official?.release_date),
    parameter_count: model.parameter_count ?? official?.parameter_count ?? '',
    input_modalities: [
      ...(model.input_modalities ?? official?.input_modalities ?? []),
    ],
    output_modalities: [
      ...(model.output_modalities ?? official?.output_modalities ?? []),
    ],
    capabilities: [...(model.capabilities ?? official?.capabilities ?? [])],
    sources: [...(official?.sources ?? [])],
    checked_at: official?.checked_at ?? '',
  }
}

const TOKEN_FORMAT = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 3,
})

/** Compact display; the UI also exposes the exact integer in its title. */
export function formatTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return '—'
  if (tokens >= 1_000_000) return `${TOKEN_FORMAT.format(tokens / 1_000_000)}M`
  if (tokens >= 1_000) return `${TOKEN_FORMAT.format(tokens / 1_000)}K`
  return TOKEN_FORMAT.format(tokens)
}

/** Preserve source date precision and avoid time-zone shifts. */
export function formatYearMonth(value: string): string {
  const normalized = validDate(value)
  if (!normalized) return '—'
  const [year, month, day = 1] = normalized.split('-').map(Number)
  const options: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: 'short',
    timeZone: 'UTC',
  }
  if (normalized.length === 10) options.day = 'numeric'
  return new Date(Date.UTC(year, month - 1, day)).toLocaleDateString(
    undefined,
    options
  )
}

export type ApiInfo = {
  vendor: string
  vendor_label: string
  tokenizer: string
  tokenizer_note?: string
  license: string
  license_kind: 'proprietary' | 'open' | 'open-weight' | 'unknown'
  data_retention_days: number | null
  training_opt_out: boolean | null
  homepage?: string
}

/** Routing to a model does not establish a channel's tokenizer, license or privacy policy. */
export function inferApiInfo(model: PricingModel): ApiInfo {
  const official =
    VERIFIED_MODEL_METADATA[(model.model_name || '').trim().toLowerCase()]
  return {
    vendor: official?.vendor_label ?? 'unknown',
    vendor_label: model.vendor_name || official?.vendor_label || 'Unknown',
    tokenizer: 'Unknown',
    license: 'Unknown',
    license_kind: 'unknown',
    data_retention_days: null,
    training_opt_out: null,
    homepage: official?.sources[0],
  }
}
