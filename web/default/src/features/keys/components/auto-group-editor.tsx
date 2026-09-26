/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useState } from 'react'
import { ArrowDown, ArrowUp, Plus, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { ApiKeyGroupOption } from './api-key-group-combobox'

type Props = {
  options: ApiKeyGroupOption[]
  value: string[]
  onChange: (groups: string[]) => void
  disabled?: boolean
}
export function AutoGroupEditor(props: Props) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  function move(index: number, offset: number) {
    const next = [...props.value]
    ;[next[index], next[index + offset]] = [next[index + offset], next[index]]
    props.onChange(next)
  }
  const available = props.options.filter(
    (option) =>
      option.auto_eligible &&
      !props.value.includes(option.value) &&
      (option.value + ' ' + (option.desc || ''))
        .toLowerCase()
        .includes(search.toLowerCase())
  )
  return (
    <div className='space-y-3' data-testid='auto-group-editor'>
      <p className='text-sm font-medium'>{t('Auto group priority')}</p>
      <p className='text-muted-foreground text-xs'>
        {t('Charged at the actual group rate')}
      </p>
      <ol className='space-y-2'>
        {props.value.map((name, index) => {
          const option = props.options.find((item) => item.value === name)
          return (
            <li
              key={name}
              className='flex items-center gap-2 rounded-lg border p-2'
              data-auto-group={name}
            >
              <span className='text-muted-foreground text-xs'>{index + 1}</span>
              <div className='min-w-0 flex-1'>
                <div className='text-sm font-medium break-words'>{name}</div>
                <div className='text-muted-foreground text-xs break-words'>
                  {option?.desc}
                </div>
                <div className='mt-1 flex flex-wrap gap-1'>
                  {option?.auto_types?.map((type) => (
                    <span
                      key={type}
                      className='bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-[10px]'
                    >
                      {t(type === 'image' ? 'Image' : 'Text')}
                    </span>
                  ))}
                </div>
                {option?.auto_eligible ? (
                  <span className='text-xs'>
                    {option.ratio}x {t('Ratio')}
                  </span>
                ) : (
                  <span className='text-destructive text-xs'>
                    {option
                      ? t('No available groups')
                      : t('Group removed or access revoked')}
                  </span>
                )}
              </div>
              <div className='flex shrink-0 gap-0.5'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8'
                  aria-label={t('Move up') + ': ' + name}
                  disabled={props.disabled || index === 0}
                  onClick={() => move(index, -1)}
                >
                  <ArrowUp className='size-4' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8'
                  aria-label={t('Move down') + ': ' + name}
                  disabled={props.disabled || index === props.value.length - 1}
                  onClick={() => move(index, 1)}
                >
                  <ArrowDown className='size-4' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8'
                  aria-label={t('Remove') + ': ' + name}
                  disabled={props.disabled}
                  onClick={() =>
                    props.onChange(
                      props.value.filter((group) => group !== name)
                    )
                  }
                >
                  <X className='size-4' />
                </Button>
              </div>
            </li>
          )
        })}
      </ol>
      {props.value.length === 0 && (
        <p className='text-muted-foreground text-xs'>{t('Select a group')}</p>
      )}
      <Input
        aria-label={t('Search...')}
        placeholder={t('Search...')}
        value={search}
        onChange={(event) => setSearch(event.target.value)}
        disabled={props.disabled}
      />
      <div className='max-h-48 space-y-1 overflow-y-auto rounded-lg border p-1'>
        {available.map((option) => (
          <Button
            key={option.value}
            type='button'
            variant='ghost'
            className='h-auto w-full justify-start gap-2 py-2 text-left whitespace-normal'
            disabled={props.disabled}
            onClick={() => props.onChange([...props.value, option.value])}
          >
            <Plus className='size-4 shrink-0' />
            <span className='min-w-0 flex-1 break-words'>
              {option.label}
              <span className='text-muted-foreground block text-xs'>
                {option.desc}
              </span>
              <span className='text-muted-foreground block text-xs'>
                {option.auto_types
                  ?.map((type) => t(type === 'image' ? 'Image' : 'Text'))
                  .join(' / ')}
              </span>
            </span>
            <span className='shrink-0 text-xs'>{option.ratio}x</span>
          </Button>
        ))}
        {available.length === 0 && (
          <p className='text-muted-foreground p-2 text-xs'>
            {t('No group found.')}
          </p>
        )}
      </div>
    </div>
  )
}
