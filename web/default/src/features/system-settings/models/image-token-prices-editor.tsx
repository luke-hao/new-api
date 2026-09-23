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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { safeJsonParse } from '../utils/json-parser'

const lanes = [
  ['input', 'Text input'],
  ['output', 'Text output'],
  ['image_input', 'Image input'],
  ['image_output', 'Image output'],
  ['cached_input', 'Cached text input'],
  ['cached_image_input', 'Cached image input'],
  ['cache_creation', 'Cache creation'],
] as const
type Lane = (typeof lanes)[number][0]
type Prices = Record<string, Record<string, Partial<Record<Lane, number>>>>
type Props = {
  value: string
  onChange: (value: string) => void
  groups: string[]
  modelsByGroup: Record<string, string[]>
  tokenGroups: string
  onTokenGroupsChange: (value: string) => void
  groupRatios: string
}

export function ImageTokenPricesEditor(props: Props) {
  const { t } = useTranslation()
  const prices = useMemo(
    () => safeJsonParse<Prices>(props.value, { fallback: {}, silent: true }),
    [props.value]
  )
  const enabled = useMemo(
    () =>
      safeJsonParse<string[]>(props.tokenGroups, {
        fallback: [],
        silent: true,
      }),
    [props.tokenGroups]
  )
  const ratios = useMemo(
    () =>
      safeJsonParse<Record<string, number>>(props.groupRatios, {
        fallback: {},
        silent: true,
      }),
    [props.groupRatios]
  )
  const groups = props.groups.filter(
    (group) => props.modelsByGroup[group]?.length || enabled.includes(group)
  )
  const [selectedGroup, setSelectedGroup] = useState('')
  const group = groups.includes(selectedGroup)
    ? selectedGroup
    : (enabled.find((item) => groups.includes(item)) ?? groups[0] ?? '')
  const models = Array.from(
    new Set([
      ...(props.modelsByGroup[group] ?? []),
      ...Object.keys(prices[group] ?? {}),
    ])
  ).sort()
  const [selectedModel, setSelectedModel] = useState('')
  const model = models.includes(selectedModel)
    ? selectedModel
    : (models[0] ?? '')
  const active = enabled.includes(group)
  const current = prices[group]?.[model] ?? {}
  const [draft, setDraft] = useState<Record<string, string>>({})
  const update = (lane: Lane, raw: string) => {
    if (!/^\d*\.?\d*$/.test(raw)) return
    setDraft((previous) => ({
      ...previous,
      [`${group}/${model}/${lane}`]: raw,
    }))
    const next = structuredClone(prices)
    next[group] ??= {}
    next[group][model] ??= {}
    if (raw === '' || raw === '.') delete next[group][model][lane]
    else next[group][model][lane] = Number(raw)
    props.onChange(JSON.stringify(next, null, 2))
  }
  return (
    <Card data-testid='image-token-prices'>
      <CardHeader>
        <CardTitle>{t('Image token prices by group')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Image token prices are saved separately for each billing group and model. Global model price changes do not overwrite them.'
          )}
        </p>
        <div className='grid gap-4 md:grid-cols-2'>
          <div className='space-y-2'>
            <Label>{t('Billing group')}</Label>
            <Select
              value={group}
              onValueChange={(value) => {
                if (value) {
                  setSelectedGroup(value)
                  setSelectedModel('')
                  setDraft({})
                }
              }}
            >
              <SelectTrigger
                className='w-full'
                aria-label={t('Image token billing group')}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {groups.map((item) => (
                    <SelectItem value={item} key={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-2'>
            <Label>{t('Model')}</Label>
            <Select
              value={model}
              onValueChange={(value) => {
                if (value) {
                  setSelectedModel(value)
                  setDraft({})
                }
              }}
            >
              <SelectTrigger
                className='w-full'
                aria-label={t('Image token model')}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {models.map((item) => (
                    <SelectItem value={item} key={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        </div>
        <label className='flex items-center gap-2'>
          <Switch
            checked={active}
            disabled={!group}
            onCheckedChange={(checked) =>
              props.onTokenGroupsChange(
                JSON.stringify(
                  checked
                    ? Array.from(new Set([...enabled, group])).sort()
                    : enabled.filter((item) => item !== group)
                )
              )
            }
          />
          {t('Bill images in this group by tokens')}
        </label>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Prices below use USD per 1M tokens, before the group multiplier. Group multiplier: {{ratio}}. User-specific group multipliers take precedence.',
            { ratio: ratios[group] ?? 1 }
          )}
        </p>
        {!active && (
          <p className='text-muted-foreground text-sm'>
            {t(
              'Token billing is off for this group. These prices are kept for later; per-image or global pricing currently applies.'
            )}
          </p>
        )}
        {model && (
          <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
            {lanes.map(([lane, title]) => {
              const id = `image-token-${lane}`
              return (
                <div className='space-y-2' key={lane}>
                  <Label htmlFor={id}>{t(title)} ($ / 1M)</Label>
                  <Input
                    id={id}
                    inputMode='decimal'
                    placeholder={t('Unset price')}
                    value={
                      draft[`${group}/${model}/${lane}`] ?? current[lane] ?? ''
                    }
                    onChange={(event) => update(lane, event.target.value)}
                  />
                </div>
              )
            })}
          </div>
        )}
        <p className='text-muted-foreground text-sm'>
          {t(
            'Fill every price field for each model used in a token billing group. Zero means free. Save group settings to apply. Per-image prices do not apply while token billing is on.'
          )}
        </p>
      </CardContent>
    </Card>
  )
}
