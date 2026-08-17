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
import { useEffect, useMemo, useRef, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
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

const MODEL_NAME = 'gpt-image-2'
const NATIVE_4K_GROUP = '生图分组-image2-4k(原生)'
const TIERS = ['1K', '2K', '4K'] as const

type Tier = (typeof TIERS)[number]
type TierPrices = Partial<Record<Tier, string>>
type PriceMap = Record<
  string,
  Record<string, Record<string, Partial<Record<Tier, number>>>>
>

type PriceRow = {
  id: string
  userGroup: string
  billingGroup: string
  prices: TierPrices
}

type ImageSizePriceOverridesEditorProps = {
  value: string
  onChange: (value: string) => void
  userGroups: string[]
  billingGroups: string[]
}

let rowCounter = 0
function createRowId() {
  rowCounter += 1
  return `isgp_${rowCounter}`
}

function parseRows(value: string): PriceRow[] {
  let parsed: PriceMap = {}
  try {
    parsed = value.trim() ? (JSON.parse(value) as PriceMap) : {}
  } catch {
    return []
  }

  const rows: PriceRow[] = []
  for (const [userGroup, usingGroups] of Object.entries(parsed)) {
    if (!usingGroups || typeof usingGroups !== 'object') continue
    for (const [billingGroup, models] of Object.entries(usingGroups)) {
      const tiers = models?.[MODEL_NAME]
      if (!tiers || typeof tiers !== 'object') continue
      const prices: TierPrices = {}
      for (const tier of TIERS) {
        const price = tiers[tier]
        if (typeof price === 'number' && Number.isFinite(price) && price >= 0) {
          prices[tier] = String(price)
        }
      }
      rows.push({ id: createRowId(), userGroup, billingGroup, prices })
    }
  }
  return rows
}

function serializeRows(rows: PriceRow[]): string {
  const result: PriceMap = {}
  for (const row of rows) {
    if (!row.userGroup || !row.billingGroup) continue
    const tiers: Partial<Record<Tier, number>> = {}
    for (const tier of TIERS) {
      if (row.billingGroup === NATIVE_4K_GROUP && tier !== '4K') continue
      const raw = row.prices[tier]
      if (raw === undefined || raw === '') continue
      const price = Number(raw)
      if (Number.isFinite(price) && price >= 0) tiers[tier] = price
    }
    if (Object.keys(tiers).length === 0) continue
    result[row.userGroup] ??= {}
    result[row.userGroup][row.billingGroup] ??= {}
    result[row.userGroup][row.billingGroup][MODEL_NAME] = tiers
  }
  return JSON.stringify(result, null, 2)
}

function uniqueOptions(options: string[], current: string) {
  return Array.from(new Set([...options, current].filter(Boolean))).sort()
}

export function ImageSizePriceOverridesEditor({
  value,
  onChange,
  userGroups,
  billingGroups,
}: ImageSizePriceOverridesEditorProps) {
  const { t } = useTranslation()
  const [rows, setRows] = useState<PriceRow[]>(() => parseRows(value))
  const lastValue = useRef(value)

  useEffect(() => {
    if (value !== lastValue.current) {
      setRows(parseRows(value))
      lastValue.current = value
    }
  }, [value])

  const usedPairs = useMemo(
    () =>
      new Set(rows.map((row) => `${row.userGroup}\u0000${row.billingGroup}`)),
    [rows]
  )
  const nextPair = useMemo(() => {
    for (const userGroup of userGroups) {
      for (const billingGroup of billingGroups) {
        if (!usedPairs.has(`${userGroup}\u0000${billingGroup}`)) {
          return { userGroup, billingGroup }
        }
      }
    }
    return null
  }, [billingGroups, usedPairs, userGroups])

  const commit = (nextRows: PriceRow[]) => {
    setRows(nextRows)
    const serialized = serializeRows(nextRows)
    lastValue.current = serialized
    onChange(serialized)
  }

  const updateRow = (index: number, patch: Partial<PriceRow>) => {
    const nextRows = rows.map((row, rowIndex) => {
      if (rowIndex !== index) return row
      const next = { ...row, ...patch }
      if (next.billingGroup === NATIVE_4K_GROUP) {
        next.prices = { ...next.prices, '1K': '', '2K': '' }
      }
      return next
    })
    commit(nextRows)
  }

  const updatePrice = (index: number, tier: Tier, raw: string) => {
    if (raw !== '' && !/^\d*\.?\d*$/.test(raw)) return
    const row = rows[index]
    updateRow(index, { prices: { ...row.prices, [tier]: raw } })
  }

  const addRow = () => {
    if (!nextPair) return
    setRows((current) => [
      ...current,
      {
        id: createRowId(),
        userGroup: nextPair.userGroup,
        billingGroup: nextPair.billingGroup,
        prices: {},
      },
    ])
  }

  return (
    <Card>
      <CardHeader className='flex-row items-center justify-between gap-3 border-b'>
        <CardTitle className='text-base'>
          {t('Image')} {t('Fixed price')}
        </CardTitle>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={addRow}
          disabled={!nextPair}
        >
          <Plus className='mr-2 h-4 w-4' />
          {t('Add')}
        </Button>
      </CardHeader>
      <CardContent className='space-y-3 pt-4'>
        {rows.length === 0 ? (
          <div className='text-muted-foreground py-6 text-center text-sm'>
            {t('No data')}
          </div>
        ) : (
          rows.map((row, index) => {
            const native4K = row.billingGroup === NATIVE_4K_GROUP
            const userOptions = uniqueOptions(userGroups, row.userGroup)
            const billingOptions = uniqueOptions(
              billingGroups,
              row.billingGroup
            )
            return (
              <div
                key={row.id}
                className='grid grid-cols-1 gap-3 rounded-md border p-3 md:grid-cols-2 xl:grid-cols-[minmax(150px,1fr)_minmax(180px,1.2fr)_150px_repeat(3,minmax(100px,0.7fr))_36px] xl:items-end'
              >
                <div className='space-y-1.5'>
                  <Label>{t('User Group')}</Label>
                  <Select
                    value={row.userGroup}
                    onValueChange={(userGroup) => {
                      if (userGroup) updateRow(index, { userGroup })
                    }}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {userOptions.map((userGroup) => (
                          <SelectItem
                            key={userGroup}
                            value={userGroup}
                            disabled={rows.some(
                              (candidate, candidateIndex) =>
                                candidateIndex !== index &&
                                candidate.userGroup === userGroup &&
                                candidate.billingGroup === row.billingGroup
                            )}
                          >
                            {userGroup}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </div>

                <div className='space-y-1.5'>
                  <Label>{t('Group')}</Label>
                  <Select
                    value={row.billingGroup}
                    onValueChange={(billingGroup) => {
                      if (billingGroup) updateRow(index, { billingGroup })
                    }}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {billingOptions.map((billingGroup) => (
                          <SelectItem
                            key={billingGroup}
                            value={billingGroup}
                            disabled={rows.some(
                              (candidate, candidateIndex) =>
                                candidateIndex !== index &&
                                candidate.userGroup === row.userGroup &&
                                candidate.billingGroup === billingGroup
                            )}
                          >
                            {billingGroup}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </div>

                <div className='space-y-1.5'>
                  <Label>{t('Model')}</Label>
                  <Input value={MODEL_NAME} disabled />
                </div>

                {TIERS.map((tier) => {
                  const disabled = native4K && tier !== '4K'
                  return (
                    <div key={tier} className='space-y-1.5'>
                      <Label>{tier} (USD)</Label>
                      <Input
                        type='text'
                        inputMode='decimal'
                        value={disabled ? '' : (row.prices[tier] ?? '')}
                        disabled={disabled}
                        aria-label={`${tier} ${t('Price')}`}
                        onChange={(event) =>
                          updatePrice(index, tier, event.target.value)
                        }
                      />
                    </div>
                  )
                })}

                <Button
                  type='button'
                  size='icon'
                  variant='ghost'
                  className='text-destructive'
                  title={t('Delete')}
                  aria-label={t('Delete')}
                  onClick={() =>
                    commit(rows.filter((_, rowIndex) => rowIndex !== index))
                  }
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            )
          })
        )}
      </CardContent>
    </Card>
  )
}
