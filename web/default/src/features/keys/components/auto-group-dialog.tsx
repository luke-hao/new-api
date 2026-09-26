/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Dialog } from '@/components/dialog'
import { batchUpdateApiKeyGroup } from '../api'
import type { ApiKey } from '../types'
import type { ApiKeyGroupOption } from './api-key-group-combobox'
import { AutoGroupEditor } from './auto-group-editor'

type Props = {
  apiKey: ApiKey
  options: ApiKeyGroupOption[]
  onClose: () => void
  onSaved: () => void
}
export function AutoGroupDialog(props: Props) {
  const { t } = useTranslation()
  const [groups, setGroups] = useState<string[]>(props.apiKey.auto_groups || [])
  const [retry, setRetry] = useState(
    props.apiKey.group === 'auto' ? props.apiKey.cross_group_retry : true
  )
  const [saving, setSaving] = useState(false)
  const valid =
    groups.length > 0 &&
    groups.every((name) =>
      props.options.some(
        (option) => option.value === name && option.auto_eligible
      )
    )
  async function save() {
    setSaving(true)
    try {
      const result = await batchUpdateApiKeyGroup({
        ids: [props.apiKey.id],
        group: 'auto',
        auto_groups: groups,
        cross_group_retry: retry,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to update API key groups'))
        return
      }
      props.onSaved()
      props.onClose()
    } catch {
      toast.error(t('Failed to update API key groups'))
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !saving) props.onClose()
      }}
      title={t('Auto group priority')}
      description={t('Charged at the actual group rate')}
      contentHeight='auto'
      contentClassName='sm:max-w-lg'
      footer={
        <>
          <Button variant='outline' disabled={saving} onClick={props.onClose}>
            {t('Cancel')}
          </Button>
          <Button disabled={!valid || saving} onClick={save}>
            {t('Save changes')}
          </Button>
        </>
      }
    >
      <AutoGroupEditor
        options={props.options}
        value={groups}
        onChange={setGroups}
        disabled={saving}
      />
      <label className='mt-4 flex items-center justify-between gap-3 text-sm'>
        {t('Cross-group retry')}
        <Switch checked={retry} onCheckedChange={setRetry} disabled={saving} />
      </label>
    </Dialog>
  )
}
