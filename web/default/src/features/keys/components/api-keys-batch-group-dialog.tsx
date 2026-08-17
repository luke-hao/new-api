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
import { useQuery } from '@tanstack/react-query'
import { type Table } from '@tanstack/react-table'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserGroups } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Dialog } from '@/components/dialog'
import { batchUpdateApiKeyGroup } from '../api'
import { type ApiKey } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { useApiKeys } from './api-keys-provider'

type ApiKeysBatchGroupDialogProps<TData> = {
  open: boolean
  onOpenChange: (open: boolean) => void
  table: Table<TData>
}

export function ApiKeysBatchGroupDialog<TData>({
  open,
  onOpenChange,
  table,
}: ApiKeysBatchGroupDialogProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const [group, setGroup] = useState('')
  const [crossGroupRetry, setCrossGroupRetry] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const { data: groupsData, isLoading: isLoadingGroups } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    enabled: open,
    staleTime: 0,
  })

  const groups = useMemo<ApiKeyGroupOption[]>(() => {
    return Object.entries(groupsData?.data || {}).map(([key, info]) => ({
      value: key,
      label: key,
      desc: info.desc || key,
      ratio: info.ratio,
    }))
  }, [groupsData])

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setGroup('')
      setCrossGroupRetry(true)
    }
    onOpenChange(nextOpen)
  }

  const handleSubmit = async () => {
    if (!group || selectedRows.length === 0) return

    setIsSubmitting(true)
    try {
      const ids = selectedRows.map((row) => (row.original as ApiKey).id)
      const result = await batchUpdateApiKeyGroup({
        ids,
        group,
        cross_group_retry: group === 'auto' && crossGroupRetry,
      })

      if (result.success) {
        toast.success(
          t('Successfully updated the group for {{count}} API key(s)', {
            count: result.data ?? 0,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        handleOpenChange(false)
      } else {
        toast.error(result.message || t('Failed to update API key groups'))
      }
    } catch {
      toast.error(t('Failed to update API key groups'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Change API key group')}
      description={t('Change the group for {{count}} selected API key(s).', {
        count: selectedRows.length,
      })}
      contentHeight='auto'
      contentClassName='sm:max-w-lg'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={
              isSubmitting ||
              isLoadingGroups ||
              !group ||
              selectedRows.length === 0
            }
          >
            {isSubmitting ? (
              <Loader2 className='mr-2 size-4 animate-spin' />
            ) : null}
            {isSubmitting ? t('Saving...') : t('Change group')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-2'>
        <div className='space-y-2'>
          <Label>{t('Group')}</Label>
          {isLoadingGroups ? (
            <div className='text-muted-foreground flex min-h-20 items-center justify-center rounded-lg border text-sm'>
              <Loader2 className='mr-2 size-4 animate-spin' />
              {t('Loading...')}
            </div>
          ) : groups.length > 0 ? (
            <ApiKeyGroupCombobox
              options={groups}
              value={group}
              onValueChange={setGroup}
              placeholder={t('Select a group')}
              disabled={isSubmitting}
            />
          ) : (
            <div className='text-muted-foreground rounded-lg border px-4 py-6 text-center text-sm'>
              {t('No available groups')}
            </div>
          )}
        </div>

        {group === 'auto' ? (
          <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
            <div className='space-y-1'>
              <Label htmlFor='batch-cross-group-retry'>
                {t('Cross-group retry')}
              </Label>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'When enabled, if channels in the current group fail, it will try channels in the next group in order.'
                )}
              </p>
            </div>
            <Switch
              id='batch-cross-group-retry'
              checked={crossGroupRetry}
              onCheckedChange={setCrossGroupRetry}
              disabled={isSubmitting}
            />
          </div>
        ) : null}
      </div>
    </Dialog>
  )
}
