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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPricing } from '@/features/pricing/api'
import type { HomeModelFamily, HomeModelSummary } from '../types'

type CuratedModel = {
  source: string
  label: string
}

type FamilyDefinition = Omit<HomeModelFamily, 'models'> & {
  models: CuratedModel[]
}

const FAMILY_DEFINITIONS: FamilyDefinition[] = [
  {
    id: 'codex',
    title: 'GPT / Codex',
    descriptionKey: 'home.catalog.codex',
    iconName: 'OpenAI',
    tone: 'blue',
    models: [
      { source: 'gpt-5.6-sol', label: 'GPT-5.6 Sol' },
      { source: 'gpt-5.6-terra', label: 'GPT-5.6 Terra' },
      { source: 'gpt-5.6-luna', label: 'GPT-5.6 Luna' },
      { source: 'gpt-5.5', label: 'GPT-5.5' },
      { source: 'gpt-5.4', label: 'GPT-5.4' },
      { source: 'gpt-5.4-mini', label: 'GPT-5.4 Mini' },
    ],
  },
  {
    id: 'claude',
    title: 'Claude',
    descriptionKey: 'home.catalog.claude',
    iconName: 'Claude.Color',
    tone: 'amber',
    models: [
      { source: 'claude-fable-5', label: 'Claude Fable 5' },
      { source: 'claude-sonnet-5', label: 'Claude Sonnet 5' },
      { source: 'claude-opus-5', label: 'Claude Opus 5' },
      { source: 'claude-opus-4-8', label: 'Claude Opus 4.8' },
      { source: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6' },
      { source: 'claude-haiku-4-5-20251001', label: 'Claude Haiku 4.5' },
    ],
  },
  {
    id: 'gemini',
    title: 'Gemini',
    descriptionKey: 'home.catalog.gemini',
    iconName: 'Gemini.Color',
    tone: 'cyan',
    models: [
      { source: 'gemini-3.7-flash', label: 'Gemini 3.7 Flash' },
      { source: 'gemini-3.6-flash', label: 'Gemini 3.6 Flash' },
      { source: 'gemini-3.5-flash', label: 'Gemini 3.5 Flash' },
      { source: 'gemini-3.1-pro-preview', label: 'Gemini 3.1 Pro' },
    ],
  },
  {
    id: 'grok',
    title: 'Grok / xAI',
    descriptionKey: 'home.catalog.grok',
    iconName: 'XAI',
    tone: 'slate',
    models: [
      { source: 'grok-4.6', label: 'Grok 4.6' },
      { source: 'grok-4.5', label: 'Grok 4.5' },
      { source: 'grok-video-1.5（按秒）', label: 'Grok Video 1.5' },
      {
        source: 'grok-imagine-video-1.5-preview',
        label: 'Grok Imagine Video',
      },
    ],
  },
  {
    id: 'image',
    title: 'Image',
    descriptionKey: 'home.catalog.image',
    iconName: 'OpenAI',
    tone: 'rose',
    models: [
      { source: 'gpt-image-2', label: 'GPT Image 2' },
      { source: 'nano-banana-2', label: 'Nano Banana 2' },
      { source: 'nano-banana-pro', label: 'Nano Banana Pro' },
    ],
  },
  {
    id: 'video',
    title: 'Video',
    descriptionKey: 'home.catalog.video',
    iconName: 'Kling.Color',
    tone: 'mint',
    models: [
      { source: 'wang-3.0-720p', label: 'Wang 3.0' },
      { source: 'happyhorse-1.1-t2v-1080p', label: 'HappyHorse 1.1' },
      { source: '官方h3-1080p', label: 'H3 Video' },
      { source: 'sd2.5-720均衡版', label: 'SD 2.5' },
    ],
  },
]

const FALLBACK_MODELS: Record<string, string[]> = Object.fromEntries(
  FAMILY_DEFINITIONS.map((family) => [
    family.id,
    family.models.slice(0, 4).map((model) => model.label),
  ])
)

export function useHomeModelCatalog(): HomeModelSummary {
  const { data } = useQuery({
    queryKey: ['pricing'],
    queryFn: getPricing,
    staleTime: 5 * 60 * 1000,
  })

  return useMemo(() => {
    const availableNames = new Set(
      (data?.data ?? []).map((model) => model.model_name.toLowerCase())
    )
    const total = data?.data?.length ?? 0

    const families = FAMILY_DEFINITIONS.map((family) => {
      const liveModels = family.models
        .filter((model) => availableNames.has(model.source.toLowerCase()))
        .map((model) => model.label)

      return {
        id: family.id,
        title: family.title,
        descriptionKey: family.descriptionKey,
        iconName: family.iconName,
        tone: family.tone,
        models: liveModels.length > 0 ? liveModels : FALLBACK_MODELS[family.id],
      }
    })

    return {
      total,
      displayTotal: total > 0 ? `${total}+` : '80+',
      isLive: total > 0,
      families,
    }
  }, [data])
}
