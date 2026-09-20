/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { LockKeyhole, LockKeyholeOpen, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { updateChannelGroupRouting } from '../api'
import { channelsQueryKeys } from '../lib'
import type { Channel, ChannelGroupRoutingUpdateItem } from '../types'
import { NumericSpinnerInput } from './numeric-spinner-input'

export function ChannelModelPriorityCell(props: {
  channel: Channel
  group: string
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const channel = props.channel
  const mutation = useMutation({
    mutationFn: async (
      patch: Omit<ChannelGroupRoutingUpdateItem, 'channel_id'>
    ) => {
      const r = await updateChannelGroupRouting({
        group: props.group,
        model: channel.routing_model,
        mode: 'manual',
        updates: [{ channel_id: channel.id, ...patch }],
      })
      if (!r.success)
        throw new Error(r.message || t('channels.modelRouting.saveFailed'))
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.lists(),
      })
      void queryClient.invalidateQueries({
        queryKey: ['channel-group-stability', props.group],
      })
    },
    onError: (e) => toast.error(e.message),
  })
  const disabled = mutation.isPending || channel.group_priority_locked
  const source =
    channel.priority_source === 'model'
      ? 'modelOverride'
      : channel.priority_source === 'group'
        ? 'groupDefault'
        : 'channelDefault'
  return (
    <div className='flex flex-col gap-1'>
      <div className='flex flex-wrap items-center gap-1'>
        <Button
          size='icon-sm'
          variant='ghost'
          disabled={disabled}
          aria-label={t(
            channel.priority_locked
              ? 'channels.modelRouting.unlock'
              : 'channels.modelRouting.lock'
          )}
          title={t(
            channel.group_priority_locked
              ? 'channels.modelRouting.groupLocked'
              : 'channels.modelRouting.lock'
          )}
          onClick={() =>
            mutation.mutate({ priority_locked: !channel.priority_locked })
          }
        >
          {channel.priority_locked ? (
            <LockKeyhole className='size-3.5' />
          ) : (
            <LockKeyholeOpen className='size-3.5' />
          )}
        </Button>
        <NumericSpinnerInput
          value={channel.effective_priority ?? channel.priority ?? 0}
          min={-999}
          disabled={disabled}
          onChange={(priority) => mutation.mutate({ priority })}
        />
        {channel.priority_overridden && (
          <Button
            variant='outline'
            size='xs'
            disabled={disabled || channel.priority_locked}
            onClick={() => mutation.mutate({ inherit_priority: true })}
          >
            <RotateCcw className='size-3' />
            {t('channels.modelRouting.restore')}
          </Button>
        )}
      </div>
      <div className='text-muted-foreground flex flex-wrap items-center gap-1 text-[10px]'>
        <span>{t('channels.modelRouting.' + source)}</span>
        {channel.model_test_result && (
          <span title={channel.model_test_message}>
            · {t('channels.modelRouting.probe.' + channel.model_test_result)}
            {channel.model_response_time
              ? ' ' + channel.model_response_time + 'ms'
              : ''}
          </span>
        )}
      </div>
    </div>
  )
}
