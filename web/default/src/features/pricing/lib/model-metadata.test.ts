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
import assert from 'node:assert/strict'
// Locale resources use a translation namespace; root-level keys are ignored by i18next.
import { readFileSync } from 'node:fs'
import { describe, it as test } from 'node:test'
import type { PricingModel } from '../types'
import {
  formatTokenCount,
  formatYearMonth,
  inferApiInfo,
  inferModelMetadata,
} from './model-metadata'
import { VERIFIED_MODEL_METADATA } from './official-model-metadata'

function model(name: string, fields: Partial<PricingModel> = {}): PricingModel {
  return {
    model_name: name,
    supported_endpoint_types: ['openai'],
    ...fields,
  } as PricingModel
}

describe('verified model metadata', () => {
  test('GPT-6 Astra uses its exact official specification and launch date', () => {
    const actual = inferModelMetadata(model('gpt-6-astra'))
    assert.equal(actual.context_length, 1_050_000)
    assert.equal(actual.max_output_tokens, 128_000)
    assert.equal(actual.knowledge_cutoff, '2026-04-30')
    assert.equal(actual.release_date, '2026-09-03')
    assert.deepEqual(actual.input_modalities, ['text', 'image'])
    assert.deepEqual(actual.output_modalities, ['text'])
    assert.equal(
      actual.sources[0],
      'https://developers.openai.com/api/docs/models/gpt-6-astra'
    )
  })

  test('GPT families do not inherit the old generic 128K/16K fallback', () => {
    for (const name of [
      'gpt-5.4',
      'gpt-5.5',
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6-luna',
    ]) {
      assert.equal(inferModelMetadata(model(name)).context_length, 1_050_000)
      assert.equal(inferModelMetadata(model(name)).max_output_tokens, 128_000)
    }
    assert.equal(
      inferModelMetadata(model('gpt-5.4-mini')).context_length,
      400_000
    )
  })

  test('Claude models keep different context limits and real release dates, not snapshot dates', () => {
    const older = inferModelMetadata(model('claude-opus-4-5-20251101'))
    assert.equal(older.context_display, '200K')
    assert.equal(older.max_output_display, '64K')
    assert.equal(older.release_date, '2025-11-24')
    assert.equal(
      inferModelMetadata(model('claude-opus-5')).max_output_display,
      '128K'
    )
    assert.equal(
      inferModelMetadata(model('claude-haiku-4-5-20251001')).knowledge_cutoff,
      '2025-02'
    )
  })

  test('Gemini and image generation models have model-specific modalities', () => {
    const gemini = inferModelMetadata(model('gemini-3.8-flash'))
    assert.equal(gemini.max_output_tokens, 65_536)
    assert.ok(gemini.input_modalities.includes('audio'))
    assert.ok(gemini.input_modalities.includes('video'))
    assert.equal(gemini.knowledge_cutoff, '')
    assert.deepEqual(
      inferModelMetadata(model('gpt-image-2')).output_modalities,
      ['image']
    )
    assert.deepEqual(
      inferModelMetadata(model('grok-imagine-video-1.5-preview'))
        .output_modalities,
      ['video']
    )
  })

  test('Chinese models use verified limits and no invented cutoff', () => {
    const qwen = inferModelMetadata(model('qwen3.8-max'))
    assert.equal(qwen.context_length, 1_000_000)
    assert.equal(qwen.max_output_tokens, 131_072)
    assert.deepEqual(qwen.input_modalities, ['text', 'image', 'video'])
    assert.deepEqual(inferModelMetadata(model('glm-5.3')).input_modalities, [
      'text',
    ])
    assert.ok(
      inferModelMetadata(model('glm-5.3-flash')).input_modalities.includes(
        'image'
      )
    )
    assert.equal(
      inferModelMetadata(model('deepseek-v4-pro-0813')).max_output_display,
      '384K'
    )
    assert.equal(
      inferModelMetadata(model('deepseek-v4-pro-0813')).release_date,
      '2026-08-13'
    )
    assert.equal(inferModelMetadata(model('MiniMax-M3')).max_output_tokens, 0)
    assert.equal(
      inferModelMetadata(model('kimi-k3')).max_output_tokens,
      1_048_576
    )
  })

  test('unverified names, suffixes and channel aliases never get guessed facts', () => {
    for (const name of [
      'unknown-model',
      'gpt-6-astra-custom',
      'claude-future',
      'qwen-3.8-max',
      'sd2.0-720mini',
      'codex-auto-review',
    ]) {
      const value = inferModelMetadata(
        model(name, { tags: 'vision reasoning', cache_ratio: 0.1 })
      )
      assert.equal(value.context_length, 0)
      assert.equal(value.max_output_tokens, 0)
      assert.equal(value.knowledge_cutoff, '')
      assert.equal(value.release_date, '')
      assert.equal(value.parameter_count, '')
      assert.deepEqual(value.input_modalities, [])
      assert.deepEqual(value.output_modalities, [])
      assert.deepEqual(value.capabilities, [])
      assert.deepEqual(value.sources, [])
    }
  })

  test('explicit metadata overrides official values, including deliberate empty/zero fields', () => {
    const value = inferModelMetadata(
      model('claude-opus-5', {
        context_length: 0,
        max_output_tokens: 42,
        release_date: '',
        input_modalities: [],
        capabilities: [],
      })
    )
    assert.equal(value.context_length, 0)
    assert.equal(value.context_display, '')
    assert.equal(value.max_output_tokens, 42)
    assert.equal(value.max_output_display, '')
    assert.equal(value.release_date, '')
    assert.deepEqual(value.input_modalities, [])
    assert.deepEqual(value.capabilities, [])
  })

  test('invalid explicit numbers and dates remain unknown', () => {
    const value = inferModelMetadata(
      model('gpt-6-astra', {
        context_length: NaN,
        max_output_tokens: -1,
        release_date: '2026-02-30',
        knowledge_cutoff: '2026-13',
      })
    )
    assert.equal(value.context_length, 0)
    assert.equal(value.max_output_tokens, 0)
    assert.equal(value.release_date, '')
    assert.equal(value.knowledge_cutoff, '')
  })

  test('resolved arrays cannot mutate the shared source catalog', () => {
    const value = inferModelMetadata(model('gpt-6-astra'))
    value.input_modalities.length = 0
    value.sources.length = 0
    assert.deepEqual(
      inferModelMetadata(model('gpt-6-astra')).input_modalities,
      ['text', 'image']
    )
    assert.ok(inferModelMetadata(model('gpt-6-astra')).sources.length > 0)
  })

  test('every catalog entry records official HTTPS sources and a verification date', () => {
    assert.equal(Object.keys(VERIFIED_MODEL_METADATA).length, 46)
    for (const entry of Object.values(VERIFIED_MODEL_METADATA)) {
      assert.equal(entry.checked_at, '2026-09-07')
      assert.ok(entry.sources.length > 0)
      for (const source of entry.sources)
        assert.equal(new URL(source).protocol, 'https:')
    }
  })

  test('privacy, license and tokenizer claims are not inferred from a model name', () => {
    for (const name of ['gpt-6-astra', 'claude-opus-5', 'unknown']) {
      const api = inferApiInfo(model(name))
      assert.equal(api.data_retention_days, null)
      assert.equal(api.training_opt_out, null)
      assert.equal(api.tokenizer, 'Unknown')
      assert.equal(api.license_kind, 'unknown')
    }
    assert.equal(inferApiInfo(model('sd-custom')).vendor_label, 'Unknown')
  })
})

describe('specification formatting', () => {
  test('keeps 1.05M precise instead of rounding it to 1.1M', () => {
    assert.equal(formatTokenCount(1_050_000), '1.05M')
    assert.equal(formatTokenCount(128_000), '128K')
    assert.equal(formatTokenCount(65_536), '65.536K')
    assert.equal(formatTokenCount(0), '—')
    assert.equal(formatTokenCount(Infinity), '—')
  })
  test('honors source date precision without inventing a day', () => {
    assert.ok(formatYearMonth('2026-09-03').includes('3'))
    assert.notEqual(formatYearMonth('2026-09'), formatYearMonth('2026-09-01'))
    assert.equal(formatYearMonth('2026-02-30'), '—')
    assert.equal(formatYearMonth(''), '—')
  })
})

for (const locale of ['en', 'zh', 'fr', 'ru', 'ja', 'vi']) {
  test(`specification messages resolve in the ${locale} translation namespace`, () => {
    const resource = JSON.parse(
      readFileSync(
        new URL(`../../../i18n/locales/${locale}.json`, import.meta.url),
        'utf8'
      )
    )
    for (const key of [
      'Context window',
      'Unverified specifications are shown as Unknown; channel limits may differ.',
      'Official documentation',
      'Verified on',
      'Data handling depends on the configured upstream channel.',
    ]) {
      assert.equal(typeof resource.translation[key], 'string')
      assert.ok(resource.translation[key].length > 0)
    }
    assert.deepEqual(Object.keys(resource), ['translation'])
  })
}
