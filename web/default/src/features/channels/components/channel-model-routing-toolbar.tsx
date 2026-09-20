/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Clock3,
  DollarSign,
  ListTree,
  Loader2,
  Play,
  ShieldCheck,
  TimerReset,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getChannelGroupStability,
  updateChannelGroupStability,
  runChannelGroupStability,
} from '../api'
import { channelsQueryKeys, rankGroupChannelsByLowestPrice } from '../lib'
import type { ChannelGroupStabilityConfig } from '../types'
import { ChannelGroupPriorityActions } from './channel-group-priority-actions'

const statusKeys: Record<string, string> = {
  never: 'channels.modelRouting.never',
  healthy: 'channels.modelRouting.healthy',
  reranked: 'channels.modelRouting.reranked',
  unchanged: 'channels.modelRouting.unchanged',
  all_failed: 'channels.modelRouting.allFailed',
  no_channels: 'channels.modelRouting.noChannels',
  unsupported: 'channels.modelRouting.unsupported',
  error: 'channels.modelRouting.error',
}
function timestamp(value: number) {
  return value > 0 ? new Date(value).toLocaleString() : '-'
}

type Props = {
  group: string
  model: string
  onModelChange: (model: string) => void
}
export function ChannelModelRoutingToolbar(props: Props) {
  const { t } = useTranslation()
  const [overview, setOverview] = useState(false)
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['channel-group-stability', props.group],
    queryFn: async () => {
      const r = await getChannelGroupStability(props.group)
      if (!r.success || !r.data)
        throw new Error(r.message || 'Failed to load stability')
      return r.data
    },
    enabled: Boolean(props.group),
    refetchInterval: 5000,
  })
  const run = useMutation({
    mutationFn: async () => {
      const r = await runChannelGroupStability(props.group)
      if (!r.success) throw new Error(r.message)
      return r
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['channel-group-stability', props.group],
      })
      toast.success(t('channels.modelRouting.started'))
    },
    onError: (e) => toast.error(e.message),
  })
  if (!props.group) return null
  const models = query.data?.models ?? []
  return (
    <div className='flex max-w-full min-w-0 flex-wrap items-center gap-2'>
      <div className='w-56 max-w-full'>
        <Combobox
          options={[
            { label: t('channels.modelRouting.allModels'), value: '' },
            ...models.map((m) => ({
              label: m.model ?? '',
              value: m.model ?? '',
            })),
          ]}
          value={props.model}
          onValueChange={(v) => props.onModelChange(v ?? '')}
          placeholder={t('channels.modelRouting.allModels')}
          searchPlaceholder={t('channels.modelRouting.selectModel')}
          allowCustomValue={false}
          className='h-7 w-full text-xs'
        />
      </div>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              size='icon-sm'
              variant='outline'
              aria-label={t('channels.modelRouting.overview')}
              onClick={() => setOverview(true)}
            />
          }
        >
          <ListTree className='size-4' />
        </TooltipTrigger>
        <TooltipContent>{t('channels.modelRouting.overview')}</TooltipContent>
      </Tooltip>
      {props.model ? (
        <ModelStabilityActions
          key={props.group + ':' + props.model}
          group={props.group}
          model={props.model}
        />
      ) : (
        <ChannelGroupPriorityActions
          key={props.group}
          selectedGroup={props.group}
        />
      )}
      {query.isError && (
        <span className='text-destructive text-xs'>
          {t('channels.modelRouting.loadFailed')}
        </span>
      )}
      <Dialog open={overview} onOpenChange={setOverview}>
        <DialogContent className='max-h-[85dvh] w-[calc(100%-2rem)] overflow-y-auto sm:max-w-5xl'>
          <DialogHeader>
            <DialogTitle>
              {props.group} · {t('channels.modelRouting.overview')}
            </DialogTitle>
          </DialogHeader>
          <div className='flex justify-end'>
            <Button
              size='sm'
              variant='outline'
              disabled={run.isPending || query.data?.running}
              onClick={() => run.mutate()}
            >
              <Play />
              {t('channels.modelRouting.runAll')}
            </Button>
          </div>
          <div className='overflow-x-auto'>
            <table className='w-full min-w-[680px] text-left text-sm'>
              <thead>
                <tr className='border-b'>
                  {['model', 'primary', 'latency', 'state', 'last', 'next'].map(
                    (k) => (
                      <th className='px-2 py-2 font-medium' key={k}>
                        {t('channels.modelRouting.' + k)}
                      </th>
                    )
                  )}
                </tr>
              </thead>
              <tbody>
                {models.map((m) => (
                  <tr className='border-b' key={m.model}>
                    <td className='px-2 py-2'>
                      <button
                        className='text-primary max-w-64 text-left break-words hover:underline'
                        onClick={() => {
                          props.onModelChange(m.model ?? '')
                          setOverview(false)
                        }}
                      >
                        {m.model}
                      </button>
                    </td>
                    <td className='px-2 py-2'>
                      {m.last_primary_channel_id
                        ? '#' + m.last_primary_channel_id
                        : '-'}
                    </td>
                    <td className='px-2 py-2 tabular-nums'>
                      {m.last_primary_latency_ms
                        ? m.last_primary_latency_ms + 'ms'
                        : '-'}
                    </td>
                    <td className='px-2 py-2' title={m.last_message}>
                      {m.running
                        ? t('channels.modelRouting.running')
                        : !m.enabled
                          ? t('channels.modelRouting.paused')
                          : t(
                              statusKeys[m.last_result] ??
                                'channels.modelRouting.never'
                            )}
                    </td>
                    <td className='px-2 py-2 text-xs'>
                      {timestamp(m.last_check_at)}
                    </td>
                    <td className='px-2 py-2 text-xs'>
                      {timestamp(m.next_check_at)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {models.length === 0 && (
            <span className='text-muted-foreground py-4 text-center text-sm'>
              {t('channels.modelRouting.noModels')}
            </span>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}

const fields = [
  {
    key: 'interval_minutes',
    override: 'interval_minutes_override',
    label: 'interval',
    min: 1,
    max: 1440,
    icon: Clock3,
  },
  {
    key: 'healthy_threshold_seconds',
    override: 'healthy_threshold_seconds_override',
    label: 'healthySeconds',
    min: 1,
    max: 300,
    icon: ShieldCheck,
  },
  {
    key: 'probe_timeout_seconds',
    override: 'probe_timeout_seconds_override',
    label: 'timeoutSeconds',
    min: 2,
    max: 600,
    icon: TimerReset,
  },
] as const
function ModelStabilityActions(props: { group: string; model: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['channel-group-stability', props.group, props.model],
    queryFn: async () => {
      const r = await getChannelGroupStability(props.group, props.model)
      if (!r.success || !r.data)
        throw new Error(r.message || 'Failed to load model config')
      return r.data
    },
    refetchInterval: 5000,
  })
  const refresh = () => {
    void queryClient.invalidateQueries({
      queryKey: ['channel-group-stability', props.group],
    })
    void queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
  }
  const save = useMutation({
    mutationFn: async (config: ChannelGroupStabilityConfig) => {
      const r = await updateChannelGroupStability(config)
      if (!r.success) throw new Error(r.message)
      return r
    },
    onSuccess: (response) => {
      if (response.data)
        queryClient.setQueryData(
          ['channel-group-stability', props.group, props.model],
          response.data
        )
      refresh()
    },
    onError: (e) => toast.error(e.message),
  })
  const run = useMutation({
    mutationFn: async () => {
      const r = await runChannelGroupStability(props.group, props.model)
      if (!r.success) throw new Error(r.message)
      return r
    },
    onSuccess: () => {
      refresh()
      toast.success(t('channels.modelRouting.started'))
    },
    onError: (e) => toast.error(e.message),
  })
  const price = useMutation({
    mutationFn: () => rankGroupChannelsByLowestPrice(props.group, props.model),
    onSuccess: (r) => {
      refresh()
      if (r.failedUpdates) toast.error(t('channels.modelRouting.saveFailed'))
      else toast.success(t('channels.modelRouting.saved'))
    },
    onError: (e) => toast.error(e.message),
  })
  const status =
    query.data && save.isPending
      ? { ...query.data, ...save.variables }
      : query.data
  if (!status)
    return (
      <span className='text-muted-foreground text-xs'>
        {query.isError ? (
          t('channels.modelRouting.loadFailed')
        ) : (
          <Loader2 className='size-4 animate-spin' />
        )}
      </span>
    )
  const persist = (patch: Partial<ChannelGroupStabilityConfig>) =>
    save.mutate({
      group: props.group,
      model: props.model,
      enabled: status.enabled,
      paused: status.paused,
      interval_minutes: status.interval_minutes,
      healthy_threshold_seconds: status.healthy_threshold_seconds,
      probe_timeout_seconds: status.probe_timeout_seconds,
      interval_minutes_override: status.interval_minutes_override ?? null,
      healthy_threshold_seconds_override:
        status.healthy_threshold_seconds_override ?? null,
      probe_timeout_seconds_override:
        status.probe_timeout_seconds_override ?? null,
      ...patch,
    })
  const busy = save.isPending || run.isPending || price.isPending
  return (
    <div className='flex min-w-0 flex-wrap items-center gap-2'>
      <label className='flex h-7 items-center gap-2 text-xs'>
        <Switch
          size='sm'
          checked={!status.paused}
          disabled={save.isPending}
          onCheckedChange={(checked) => persist({ paused: !checked })}
        />
        {t('channels.modelRouting.auto')}
      </label>
      {!status.group_enabled && (
        <span className='text-muted-foreground text-xs'>
          {t('channels.modelRouting.groupPaused')}
        </span>
      )}
      {fields.map((field) => (
        <div
          key={field.key}
          className='flex h-7 items-center gap-1 rounded-md border px-2 text-xs'
        >
          <field.icon className='text-muted-foreground size-3.5' />
          <Input
            key={String(status[field.override]) + ':' + status[field.key]}
            aria-label={t('channels.modelRouting.' + field.label)}
            className='h-6 w-12 border-0 px-1 text-center text-xs shadow-none'
            type='number'
            min={field.min}
            max={field.max}
            defaultValue={status[field.key]}
            disabled={busy}
            onKeyDown={(e) => {
              if (e.key === 'Enter') e.currentTarget.blur()
            }}
            onBlur={(e) => {
              const n = Number(e.currentTarget.value)
              if (
                Number.isInteger(n) &&
                n >= field.min &&
                n <= field.max &&
                n !== status[field.key]
              )
                persist({ [field.override]: n })
            }}
          />
          <span className='whitespace-nowrap'>
            {t('channels.modelRouting.' + field.label)}
          </span>
          <label className='text-muted-foreground ml-1 flex items-center gap-1 whitespace-nowrap'>
            <input
              type='checkbox'
              className='size-3 accent-current'
              checked={status[field.override] == null}
              disabled={busy}
              aria-label={
                t('channels.modelRouting.inherit') +
                ' ' +
                t('channels.modelRouting.' + field.label)
              }
              onChange={(e) =>
                persist({
                  [field.override]: e.target.checked ? null : status[field.key],
                })
              }
            />
            {t('channels.modelRouting.inherit')}
          </label>
        </div>
      ))}
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              size='icon-sm'
              variant='outline'
              disabled={busy || status.running}
              aria-label={t('channels.modelRouting.price')}
              onClick={() => price.mutate()}
            />
          }
        >
          <DollarSign className='size-4' />
        </TooltipTrigger>
        <TooltipContent>{t('channels.modelRouting.price')}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              size='icon-sm'
              variant='outline'
              disabled={busy || status.running}
              aria-label={t('channels.modelRouting.runModel')}
              onClick={() => run.mutate()}
            />
          }
        >
          {status.running ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <Play className='size-4' />
          )}
        </TooltipTrigger>
        <TooltipContent>{t('channels.modelRouting.runModel')}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={<span className='text-muted-foreground text-xs' />}
        >
          {status.running
            ? t('channels.modelRouting.running')
            : t(
                statusKeys[status.last_result] ?? 'channels.modelRouting.never'
              )}
        </TooltipTrigger>
        <TooltipContent>
          <div>{status.last_message}</div>
          <div>
            {t('channels.modelRouting.last')}: {timestamp(status.last_check_at)}
          </div>
          <div>
            {t('channels.modelRouting.next')}: {timestamp(status.next_check_at)}
          </div>
        </TooltipContent>
      </Tooltip>
    </div>
  )
}
