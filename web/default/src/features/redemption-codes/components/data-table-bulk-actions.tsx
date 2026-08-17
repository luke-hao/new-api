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
import { useState, useMemo } from 'react'
import { type Table } from '@tanstack/react-table'
import { CircleX, Download, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { deleteInvalidRedemptions, deleteRedemptionsBatch } from '../api'
import { downloadRedemptionsForPancake } from '../lib'
import { type Redemption } from '../types'
import { useRedemptions } from './redemptions-provider'

type DataTableBulkActionsProps<TData> = {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useRedemptions()
  const [showDeleteSelectedConfirm, setShowDeleteSelectedConfirm] =
    useState(false)
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] =
    useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedRedemptions = useMemo(
    () => selectedRows.map((row) => row.original as Redemption),
    [selectedRows]
  )
  const selectedIds = useMemo(
    () => selectedRedemptions.map((redemption) => redemption.id),
    [selectedRedemptions]
  )

  const contentToCopy = useMemo(() => {
    const selectedCodes = selectedRedemptions.map((redemption) => {
      return `${redemption.name}\t${redemption.key}`
    })
    return selectedCodes.join('\n')
  }, [selectedRedemptions])

  const handleExportPancakeTxt = () => {
    if (selectedRedemptions.length === 0) {
      return
    }

    downloadRedemptionsForPancake(selectedRedemptions)
    toast.success(
      t('Exported {{count}} redemption codes for Pancake', {
        count: selectedRedemptions.length,
      })
    )
  }

  const handleDeleteInvalid = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteInvalidRedemptions()

      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} invalid redemption codes', {
            count,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteInvalidConfirm(false)
      }
    } finally {
      setIsDeleting(false)
    }
  }

  const handleDeleteSelected = async () => {
    if (selectedIds.length === 0) {
      return
    }

    setIsDeleting(true)
    try {
      const result = await deleteRedemptionsBatch({ ids: selectedIds })

      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} redemption codes', {
            count,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteSelectedConfirm(false)
      }
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName={t('redemption code')}>
        <CopyButton
          value={contentToCopy}
          variant='outline'
          size='icon'
          className='size-8'
          tooltip={t('Copy selected codes')}
          successTooltip={t('Codes copied!')}
          aria-label={t('Copy selected codes')}
        />

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleExportPancakeTxt}
                className='size-8'
                aria-label={t('Export selected as Pancake TXT')}
                title={t('Export selected as Pancake TXT')}
              />
            }
          >
            <Download />
            <span className='sr-only'>
              {t('Export selected as Pancake TXT')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Export selected as Pancake TXT')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => setShowDeleteSelectedConfirm(true)}
                className='size-8'
                aria-label={t('Delete selected redemption codes')}
                title={t('Delete selected redemption codes')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>
              {t('Delete selected redemption codes')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete selected redemption codes')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowDeleteInvalidConfirm(true)}
                className='size-8'
                aria-label={t('Delete invalid redemption codes')}
                title={t('Delete invalid redemption codes')}
              />
            }
          >
            <CircleX />
            <span className='sr-only'>{t('Delete invalid codes')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete invalid codes (used/disabled/expired)')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <ConfirmDialog
        destructive
        open={showDeleteSelectedConfirm}
        onOpenChange={setShowDeleteSelectedConfirm}
        handleConfirm={handleDeleteSelected}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Selected Redemption Codes?')}
        desc={
          <>
            {t('This will delete {{count}} selected redemption code(s).', {
              count: selectedIds.length,
            })}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete selected redemption codes')}
      />

      <ConfirmDialog
        destructive
        open={showDeleteInvalidConfirm}
        onOpenChange={setShowDeleteInvalidConfirm}
        handleConfirm={handleDeleteInvalid}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Invalid Redemption Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('used')}</strong>,{' '}
            <strong>{t('disabled')}</strong>
            {t(', and')} <strong>{t('expired')}</strong>{' '}
            {t('redemption codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Invalid')}
      />
    </>
  )
}
