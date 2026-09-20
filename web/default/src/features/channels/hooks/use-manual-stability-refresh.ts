/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useCallback, useEffect, useMemo, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getChannelGroupStability } from '../api'
import { channelsQueryKeys } from '../lib'
import { createManualStabilityRefresh } from '../lib/manual-stability-refresh'

export function useManualStabilityRefresh(group: string) {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const mounted = useRef<ReturnType<
    typeof createManualStabilityRefresh
  > | null>(null)
  const monitor = useMemo(
    () =>
      createManualStabilityRefresh({
        load: async (signal) => {
          const response = await getChannelGroupStability(
            group,
            undefined,
            signal
          )
          if (!response.success || !response.data)
            throw new Error(response.message || 'Failed to load stability')
          return response.data
        },
        update: (status) => {
          queryClient.setQueryData(['channel-group-stability', group], status)
          for (const model of status.models ?? []) {
            queryClient.setQueryData(
              ['channel-group-stability', group, model.model],
              model
            )
          }
          void queryClient.invalidateQueries({
            queryKey: channelsQueryKeys.lists(),
          })
        },
        error: () => toast.error(t('Refresh failed')),
      }),
    [group, queryClient, t]
  )

  useEffect(() => {
    mounted.current = monitor
    return () => {
      mounted.current = null
      monitor.stop()
    }
  }, [monitor])

  return useCallback(
    (model?: string) => {
      if (mounted.current === monitor && group) monitor.start(model)
    },
    [group, monitor]
  )
}
