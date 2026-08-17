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
import { useId, useState, type ChangeEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Clock3,
  DollarSign,
  Gauge,
  Loader2,
  Play,
  ShieldCheck,
  TimerReset,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getChannelGroupStability,
  runChannelGroupStability,
  updateChannelGroupStability,
} from '../api'
import { channelsQueryKeys, rankGroupChannelsByLowestPrice } from '../lib'
import type {
  ChannelGroupStabilityConfig,
  ChannelGroupStabilityStatus,
} from '../types'

const DEFAULT_INTERVAL_MINUTES = 10
const DEFAULT_HEALTHY_THRESHOLD_SECONDS = 8
const DEFAULT_PROBE_TIMEOUT_SECONDS = 20
const MIN_INTERVAL_MINUTES = 1
const MAX_INTERVAL_MINUTES = 1440
const MIN_HEALTHY_THRESHOLD_SECONDS = 1
const MAX_HEALTHY_THRESHOLD_SECONDS = 300
const MIN_PROBE_TIMEOUT_SECONDS = 2
const MAX_PROBE_TIMEOUT_SECONDS = 600

type ChannelGroupPriorityActionsProps = {
  selectedGroup?: string | null
}

type StabilityNumberField =
  | 'interval_minutes'
  | 'healthy_threshold_seconds'
  | 'probe_timeout_seconds'

function clampNumber(
  value: number,
  fallback: number,
  min: number,
  max: number
) {
  if (!Number.isFinite(value)) return fallback
  return Math.min(max, Math.max(min, Math.round(value)))
}

function getErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback
}

function statusToConfig(
  status: ChannelGroupStabilityStatus
): ChannelGroupStabilityConfig {
  return {
    group: status.group,
    enabled: status.enabled,
    interval_minutes: status.interval_minutes,
    healthy_threshold_seconds: status.healthy_threshold_seconds,
    probe_timeout_seconds: status.probe_timeout_seconds,
  }
}

function defaultConfig(group: string): ChannelGroupStabilityConfig {
  return {
    group,
    enabled: false,
    interval_minutes: DEFAULT_INTERVAL_MINUTES,
    healthy_threshold_seconds: DEFAULT_HEALTHY_THRESHOLD_SECONDS,
    probe_timeout_seconds: DEFAULT_PROBE_TIMEOUT_SECONDS,
  }
}

function formatTimestamp(value: number) {
  return value > 0 ? new Date(value).toLocaleString() : '-'
}

export function ChannelGroupPriorityActions({
  selectedGroup,
}: ChannelGroupPriorityActionsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const switchId = useId()
  const intervalInputId = useId()
  const healthyInputId = useId()
  const timeoutInputId = useId()
  const group = selectedGroup?.trim() ?? ''
  const stabilityQueryKey = ['channel-group-stability', group] as const
  const [draftOverride, setDraftOverride] =
    useState<ChannelGroupStabilityConfig | null>(null)
  const [isPriceBusy, setIsPriceBusy] = useState(false)

  const {
    data: stabilityStatus,
    isLoading: isStabilityLoading,
    refetch: refetchStability,
  } = useQuery({
    queryKey: stabilityQueryKey,
    queryFn: async () => {
      const response = await getChannelGroupStability(group)
      if (!response.success || !response.data) {
        throw new Error(response.message || 'Failed to load stability config')
      }
      return response.data
    },
    enabled: group !== '',
    refetchInterval: 5000,
  })

  const draft =
    draftOverride ??
    (stabilityStatus ? statusToConfig(stabilityStatus) : defaultConfig(group))

  const saveMutation = useMutation({
    mutationFn: async (config: ChannelGroupStabilityConfig) => {
      const response = await updateChannelGroupStability(config)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('稳定通道配置保存失败'))
      }
      return response.data
    },
    onSuccess: (status) => {
      queryClient.setQueryData(stabilityQueryKey, status)
      setDraftOverride(null)
      toast.success(t('稳定通道配置已保存'))
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('稳定通道配置保存失败')))
      setDraftOverride(null)
    },
  })

  const manualRunMutation = useMutation({
    mutationFn: async () => {
      const response = await runChannelGroupStability(group)
      if (!response.success) {
        throw new Error(response.message || t('稳定通道检测启动失败'))
      }
      return response
    },
    onSuccess: async () => {
      toast.info(t('已启动 {{group}} 分组的全组稳定性检测', { group }))
      await refetchStability()
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('稳定通道检测启动失败')))
    },
  })

  const isStableBusy = Boolean(stabilityStatus?.running)
  const isConfigBusy = saveMutation.isPending || isStabilityLoading
  const isBusy = isPriceBusy || isStableBusy || manualRunMutation.isPending

  const persistConfig = (next: ChannelGroupStabilityConfig) => {
    if (next.probe_timeout_seconds <= next.healthy_threshold_seconds) {
      toast.error(t('检测超时必须大于健康阈值'))
      setDraftOverride(null)
      return
    }
    if (
      stabilityStatus &&
      next.enabled === stabilityStatus.enabled &&
      next.interval_minutes === stabilityStatus.interval_minutes &&
      next.healthy_threshold_seconds ===
        stabilityStatus.healthy_threshold_seconds &&
      next.probe_timeout_seconds === stabilityStatus.probe_timeout_seconds
    ) {
      setDraftOverride(null)
      return
    }
    setDraftOverride(next)
    saveMutation.mutate(next)
  }

  const handleAutoToggle = (checked: boolean) => {
    if (!group || isConfigBusy) return
    persistConfig({ ...draft, group, enabled: checked })
  }

  const handleNumberChange =
    (field: StabilityNumberField) => (event: ChangeEvent<HTMLInputElement>) => {
      setDraftOverride((current) => ({
        ...(current ?? draft),
        [field]: Number(event.target.value),
      }))
    }

  const commitNumberField = (field: StabilityNumberField) => {
    const next = { ...draft, group }
    if (field === 'interval_minutes') {
      next.interval_minutes = clampNumber(
        next.interval_minutes,
        DEFAULT_INTERVAL_MINUTES,
        MIN_INTERVAL_MINUTES,
        MAX_INTERVAL_MINUTES
      )
    } else if (field === 'healthy_threshold_seconds') {
      next.healthy_threshold_seconds = clampNumber(
        next.healthy_threshold_seconds,
        DEFAULT_HEALTHY_THRESHOLD_SECONDS,
        MIN_HEALTHY_THRESHOLD_SECONDS,
        MAX_HEALTHY_THRESHOLD_SECONDS
      )
    } else if (field === 'probe_timeout_seconds') {
      next.probe_timeout_seconds = clampNumber(
        next.probe_timeout_seconds,
        DEFAULT_PROBE_TIMEOUT_SECONDS,
        MIN_PROBE_TIMEOUT_SECONDS,
        MAX_PROBE_TIMEOUT_SECONDS
      )
    }
    persistConfig(next)
  }

  const handlePriceSort = async () => {
    if (!group || isBusy || isConfigBusy) return
    setIsPriceBusy(true)
    toast.info(t('正在按价格排序 {{group}} 分组...', { group }))
    try {
      const result = await rankGroupChannelsByLowestPrice(group)
      await queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.lists(),
      })
      if (result.total === 0) {
        toast.info(t('当前分组没有渠道'))
      } else if (result.failedUpdates > 0) {
        toast.error(
          t('价格排序完成，但 {{count}} 个渠道优先级更新失败', {
            count: result.failedUpdates,
          })
        )
      } else if (result.priced === 0) {
        toast.info(t('未找到名称末尾带倍率数字的渠道，已将优先级设为 0'))
      } else {
        toast.success(
          t(
            '价格排序完成：{{priced}} 个参与排序，{{unpriced}} 个设为 0，已更新 {{updated}} 个优先级',
            {
              priced: result.priced,
              unpriced: result.unpriced,
              updated: result.updated,
            }
          )
        )
      }
    } catch (error) {
      toast.error(getErrorMessage(error, t('价格排序失败')))
    } finally {
      setIsPriceBusy(false)
    }
  }

  if (!group) return null

  return (
    <div className='flex flex-wrap items-center gap-1.5 sm:gap-2'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='outline'
              size='sm'
              onClick={handlePriceSort}
              disabled={isBusy || isConfigBusy}
              aria-label={t('价格最低优先级最高')}
            />
          }
        >
          {isPriceBusy ? <Loader2 className='animate-spin' /> : <DollarSign />}
          <span className='hidden sm:inline'>{t('低价优先')}</span>
        </TooltipTrigger>
        <TooltipContent>
          {t('按当前分组名称末尾的倍率排序，数字越低优先级越高。')}
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <div
              className={cn(
                'border-border bg-background flex h-7 items-center gap-2 rounded-md border px-2.5',
                isStableBusy && 'text-muted-foreground'
              )}
            />
          }
        >
          {isStableBusy ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <Gauge className='text-muted-foreground size-3.5' />
          )}
          <Label
            htmlFor={switchId}
            className='cursor-pointer text-[0.8rem] whitespace-nowrap'
          >
            {t('自动切换稳定通道')}
          </Label>
          <Switch
            id={switchId}
            size='sm'
            checked={draft.enabled}
            onCheckedChange={handleAutoToggle}
            disabled={isConfigBusy}
          />
        </TooltipTrigger>
        <TooltipContent className='max-w-80 space-y-1'>
          <div>{stabilityStatus?.last_message || t('尚未执行检测')}</div>
          <div>
            {t('上次检测')}:{' '}
            {formatTimestamp(stabilityStatus?.last_check_at ?? 0)}
          </div>
          <div>
            {t('下次检测')}:{' '}
            {formatTimestamp(stabilityStatus?.next_check_at ?? 0)}
          </div>
          {Boolean(stabilityStatus?.last_primary_channel_id) && (
            <div>
              {t('主通道')}: #{stabilityStatus?.last_primary_channel_id} ·{' '}
              {stabilityStatus?.last_primary_latency_ms}ms
            </div>
          )}
        </TooltipContent>
      </Tooltip>

      <div className='border-border bg-background flex h-7 items-center gap-1 rounded-md border px-2'>
        <Clock3 className='text-muted-foreground size-3.5' />
        <Label htmlFor={intervalInputId} className='sr-only'>
          {t('自动检测间隔（分钟）')}
        </Label>
        <Input
          id={intervalInputId}
          type='number'
          min={MIN_INTERVAL_MINUTES}
          max={MAX_INTERVAL_MINUTES}
          value={draft.interval_minutes}
          onChange={handleNumberChange('interval_minutes')}
          onBlur={() => commitNumberField('interval_minutes')}
          onKeyDown={(event) =>
            event.key === 'Enter' && event.currentTarget.blur()
          }
          disabled={isConfigBusy}
          className='h-6 w-12 rounded-sm border-0 px-1 text-center text-xs shadow-none focus-visible:ring-0'
        />
        <span className='text-muted-foreground text-xs whitespace-nowrap'>
          {t('分钟')}
        </span>
      </div>

      <Tooltip>
        <TooltipTrigger
          render={
            <div className='border-border bg-background flex h-7 items-center gap-1 rounded-md border px-2' />
          }
        >
          <ShieldCheck className='text-muted-foreground size-3.5' />
          <Label htmlFor={healthyInputId} className='sr-only'>
            {t('主通道健康阈值（秒）')}
          </Label>
          <Input
            id={healthyInputId}
            type='number'
            min={MIN_HEALTHY_THRESHOLD_SECONDS}
            max={MAX_HEALTHY_THRESHOLD_SECONDS}
            value={draft.healthy_threshold_seconds}
            onChange={handleNumberChange('healthy_threshold_seconds')}
            onBlur={() => commitNumberField('healthy_threshold_seconds')}
            onKeyDown={(event) =>
              event.key === 'Enter' && event.currentTarget.blur()
            }
            disabled={isConfigBusy}
            className='h-6 w-10 rounded-sm border-0 px-1 text-center text-xs shadow-none focus-visible:ring-0'
          />
          <span className='text-muted-foreground text-xs whitespace-nowrap'>
            {t('健康秒')}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          {t('主通道不超过此耗时则保持现有优先级')}
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <div className='border-border bg-background flex h-7 items-center gap-1 rounded-md border px-2' />
          }
        >
          <TimerReset className='text-muted-foreground size-3.5' />
          <Label htmlFor={timeoutInputId} className='sr-only'>
            {t('单渠道检测超时（秒）')}
          </Label>
          <Input
            id={timeoutInputId}
            type='number'
            min={MIN_PROBE_TIMEOUT_SECONDS}
            max={MAX_PROBE_TIMEOUT_SECONDS}
            value={draft.probe_timeout_seconds}
            onChange={handleNumberChange('probe_timeout_seconds')}
            onBlur={() => commitNumberField('probe_timeout_seconds')}
            onKeyDown={(event) =>
              event.key === 'Enter' && event.currentTarget.blur()
            }
            disabled={isConfigBusy}
            className='h-6 w-10 rounded-sm border-0 px-1 text-center text-xs shadow-none focus-visible:ring-0'
          />
          <span className='text-muted-foreground text-xs whitespace-nowrap'>
            {t('超时秒')}
          </span>
        </TooltipTrigger>
        <TooltipContent>{t('超过此时间即取消检测并视为失败')}</TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='outline'
              size='sm'
              onClick={() => manualRunMutation.mutate()}
              disabled={isBusy || isConfigBusy}
              aria-label={t('立即全组检测')}
            />
          }
        >
          {manualRunMutation.isPending || isStableBusy ? (
            <Loader2 className='animate-spin' />
          ) : (
            <Play />
          )}
          <span className='hidden sm:inline'>{t('立即全测')}</span>
        </TooltipTrigger>
        <TooltipContent>
          {t('立即检测当前分组全部启用渠道并按稳定性重新排序')}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}
