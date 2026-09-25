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
import { useEffect, useRef, useState } from 'react'
import { Check, Globe2, Gauge, Loader2, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { CopyButton } from '@/components/copy-button'
import {
  measureEndpoint,
  protocolUrl,
  type ApiProtocol,
  type LatencyResult,
} from '../lib/endpoints'
import { useApiKeys } from './api-keys-provider'

export function ApiEndpointsPanel() {
  const { t } = useTranslation()
  const { primaryEndpoint, selectedEndpoint, setSelectedEndpoint } =
    useApiKeys()
  const [protocol, setProtocol] = useState<ApiProtocol>('openai')
  const [results, setResults] = useState<Record<string, LatencyResult>>({})
  const [busy, setBusy] = useState<Record<string, boolean>>({})
  const active = useRef(new Map<string, AbortController>())
  useEffect(() => {
    const controllers = active.current
    return () => {
      for (const controller of controllers.values()) controller.abort()
    }
  }, [])
  const origins = [...new Set([primaryEndpoint, 'https://kele520.com'])]
  const allSucceeded =
    origins.length > 1 &&
    origins.every((origin) => results[origin]?.status === 'success')
  const faster = allSucceeded
    ? [...origins].sort((a, b) => results[a].ms! - results[b].ms!)[0]
    : null
  const test = async (origin: string) => {
    if (active.current.has(origin)) return
    const controller = new AbortController()
    active.current.set(origin, controller)
    setBusy((prev) => ({ ...prev, [origin]: true }))
    const result = await measureEndpoint(origin, controller.signal)
    active.current.delete(origin)
    if (!controller.signal.aborted) {
      setResults((prev) => ({ ...prev, [origin]: result }))
      setBusy((prev) => ({ ...prev, [origin]: false }))
    }
  }
  return (
    <section aria-label={t('API endpoints')} className='space-y-2.5'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='text-muted-foreground flex items-center gap-2 text-xs'>
          <Globe2 className='size-4' />
          <span>{t('API endpoints')}</span>
          <span className='hidden sm:inline'>· {t('One key, two routes')}</span>
        </div>
        <div className='flex flex-wrap items-center gap-1'>
          <Button
            size='sm'
            variant={protocol === 'openai' ? 'secondary' : 'ghost'}
            aria-pressed={protocol === 'openai'}
            onClick={() => setProtocol('openai')}
          >
            OpenAI / Codex
          </Button>
          <Button
            size='sm'
            variant={protocol === 'claude' ? 'secondary' : 'ghost'}
            aria-pressed={protocol === 'claude'}
            onClick={() => setProtocol('claude')}
          >
            Claude
          </Button>
          <Button
            size='sm'
            variant='ghost'
            disabled={Object.values(busy).some(Boolean)}
            onClick={() => {
              void Promise.all(origins.map(test))
            }}
          >
            <Gauge className='size-4' />
            {t('Test all routes')}
          </Button>
        </div>
      </div>
      <div className='grid gap-3 lg:grid-cols-2'>
        {origins.map((origin, index) => {
          const selected = selectedEndpoint === origin
          const result = results[origin]
          return (
            <div
              key={origin}
              className={cn(
                'bg-card relative min-w-0 rounded-lg border px-3 py-3 transition-colors sm:px-4',
                selected
                  ? 'border-primary/60 bg-primary/[0.06] ring-primary/20 ring-1'
                  : 'hover:border-primary/35 hover:bg-muted/30'
              )}
            >
              <button
                type='button'
                onClick={() => setSelectedEndpoint(origin)}
                aria-pressed={selected}
                aria-label={`${t(index === 0 ? 'Primary endpoint' : 'Backup endpoint')}: ${origin}`}
                className='focus-visible:outline-primary absolute inset-0 z-0 cursor-pointer rounded-lg focus-visible:outline-2 focus-visible:outline-offset-2'
              />
              <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
                <div className='flex items-center gap-2 text-sm font-medium'>
                  <span
                    className={cn(
                      'flex size-4 items-center justify-center rounded-full border',
                      selected &&
                        'border-primary bg-primary text-primary-foreground'
                    )}
                  >
                    {selected && <Check className='size-3' />}
                  </span>
                  {index === 0 ? t('Primary endpoint') : t('Backup endpoint')}
                  {selected && (
                    <span className='text-primary text-xs font-normal'>
                      {t('Active')}
                    </span>
                  )}
                </div>
                <div className='flex items-center gap-2 text-xs'>
                  {faster === origin && !Object.values(busy).some(Boolean) && (
                    <span className='inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-400'>
                      <Zap className='size-3' />
                      {t('Faster this time')}
                    </span>
                  )}
                  {result && !busy[origin] && (
                    <span
                      title={new Date(result.testedAt).toLocaleString()}
                      className={cn(
                        'tabular-nums',
                        result.status === 'success'
                          ? 'text-muted-foreground'
                          : 'text-destructive'
                      )}
                    >
                      {result.status === 'success'
                        ? result.ms + ' ms'
                        : t(
                            result.status === 'timeout'
                              ? 'Timed out'
                              : 'Connection failed'
                          )}
                    </span>
                  )}
                  <Button
                    size='icon-sm'
                    variant='ghost'
                    className='relative z-10'
                    onClick={() => void test(origin)}
                    disabled={busy[origin]}
                    aria-label={t('Test route') + ': ' + origin}
                  >
                    {busy[origin] ? (
                      <Loader2 className='size-4 animate-spin' />
                    ) : (
                      <Gauge className='size-4' />
                    )}
                  </Button>
                </div>
              </div>
              <div className='flex min-w-0 items-center justify-between gap-2'>
                <code className='text-foreground min-w-0 text-sm break-all'>
                  {protocolUrl(origin, protocol)}
                </code>
                <CopyButton
                  value={protocolUrl(origin, protocol)}
                  tooltip={t('Copy URL')}
                  aria-label={t('Copy URL')}
                  className='relative z-10 size-8 shrink-0'
                />
              </div>
            </div>
          )
        })}
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Browser round-trip latency · No model usage · Select a route for copied addresses and setup guides'
        )}
      </p>
    </section>
  )
}
