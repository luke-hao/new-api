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
import { getRouteApi } from '@tanstack/react-router'
import { useMediaQuery } from '@/hooks'
import { Download, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { parseQuotaFromDollars } from '@/lib/format'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDebouncedColumnFilter,
  useDataTable,
} from '@/components/data-table'
import {
  deleteRedemptionsBatch,
  exportRedemptions,
  getRedemptions,
} from '../api'
import { REDEMPTION_STATUS, getRedemptionStatusOptions } from '../constants'
import { downloadRedemptionsForPancake, isRedemptionExpired } from '../lib'
import type { Redemption, RedemptionFilters } from '../types'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { useRedemptionsColumns } from './redemptions-columns'
import { useRedemptions } from './redemptions-provider'

const route = getRouteApi('/_authenticated/redemption-codes/')

function isDisabledRedemptionRow(redemption: Redemption) {
  return (
    redemption.status !== REDEMPTION_STATUS.ENABLED ||
    isRedemptionExpired(redemption.expired_time, redemption.status)
  )
}

export function RedemptionsTable() {
  const { t } = useTranslation()
  const columns = useRedemptionsColumns()
  const { refreshTrigger, triggerRefresh } = useRedemptions()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [showDeleteFilteredConfirm, setShowDeleteFilteredConfirm] =
    useState(false)
  const [isExportingFiltered, setIsExportingFiltered] = useState(false)
  const [isDeletingFiltered, setIsDeletingFiltered] = useState(false)

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'name', searchKey: 'name', type: 'string' },
      { columnId: 'quota', searchKey: 'quota', type: 'string' },
    ],
  })

  const statusFilter = useMemo(
    () =>
      (columnFilters.find((f) => f.id === 'status')?.value as string[]) || [],
    [columnFilters]
  )
  const {
    value: nameFilter,
    inputValue: nameFilterInput,
    onChange: onNameFilterInputChange,
    onCompositionStart: onNameFilterCompositionStart,
    onCompositionEnd: onNameFilterCompositionEnd,
    resetInput: resetNameFilterInput,
  } = useDebouncedColumnFilter({
    columnFilters,
    columnId: 'name',
    onColumnFiltersChange,
  })
  const {
    value: quotaFilter,
    inputValue: quotaFilterInput,
    onChange: onQuotaFilterInputChange,
    onCompositionStart: onQuotaFilterCompositionStart,
    onCompositionEnd: onQuotaFilterCompositionEnd,
    resetInput: resetQuotaFilterInput,
  } = useDebouncedColumnFilter({
    columnFilters,
    columnId: 'quota',
    onColumnFiltersChange,
  })

  const quotaFilterText = quotaFilter.trim()
  const parsedQuotaFilter = useMemo(() => {
    if (quotaFilterText === '') return undefined

    const amount = Number(quotaFilterText)
    if (!Number.isFinite(amount)) return undefined

    return parseQuotaFromDollars(amount)
  }, [quotaFilterText])
  const hasInvalidQuotaFilter =
    quotaFilterText !== '' && parsedQuotaFilter === undefined

  const apiFilters = useMemo<RedemptionFilters>(() => {
    const filters: RedemptionFilters = {}
    const keyword = globalFilter?.trim()
    const name = nameFilter.trim()

    if (keyword) filters.keyword = keyword
    if (name) filters.name = name
    if (!hasInvalidQuotaFilter && parsedQuotaFilter !== undefined) {
      filters.quota = parsedQuotaFilter
    }
    if (statusFilter.length > 0) filters.status = statusFilter

    return filters
  }, [
    globalFilter,
    hasInvalidQuotaFilter,
    nameFilter,
    parsedQuotaFilter,
    statusFilter,
  ])

  const hasFilterConditions = Boolean(
    apiFilters.keyword ||
    apiFilters.name ||
    apiFilters.quota !== undefined ||
    apiFilters.status?.length
  )

  // Fetch data with React Query
  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'redemptions',
      pagination.pageIndex + 1,
      pagination.pageSize,
      apiFilters,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getRedemptions({
        ...apiFilters,
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    enabled: !hasInvalidQuotaFilter,
    placeholderData: (previousData) => previousData,
  })

  const redemptions = data?.items || []

  const { table } = useDataTable({
    data: redemptions,
    columns,
    enableRowSelection: true,
    columnFilters,
    globalFilter,
    pagination,
    globalFilterFn: (row, _columnId, filterValue) => {
      const name = String(row.getValue('name')).toLowerCase()
      const id = String(row.getValue('id'))
      const searchValue = String(filterValue).toLowerCase()

      return name.includes(searchValue) || id.includes(searchValue)
    },
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  const redemptionStatusOptions = useMemo(
    () => getRedemptionStatusOptions(t),
    [t]
  )

  const handleExportMatching = async () => {
    if (hasInvalidQuotaFilter) {
      toast.error(t('Invalid quota filter'))
      return
    }

    setIsExportingFiltered(true)
    try {
      const result = await exportRedemptions(apiFilters)
      const items = result.data?.items || []

      if (items.length === 0) {
        toast.info(t('No matching redemption codes to export'))
        return
      }

      downloadRedemptionsForPancake(
        items,
        'redemption-codes-filtered-pancake.txt'
      )
      toast.success(
        t('Exported {{count}} matching redemption codes for Pancake', {
          count: items.length,
        })
      )
    } finally {
      setIsExportingFiltered(false)
    }
  }

  const handleDeleteMatching = async () => {
    if (hasInvalidQuotaFilter) {
      toast.error(t('Invalid quota filter'))
      return
    }

    setIsDeletingFiltered(true)
    try {
      const result = await deleteRedemptionsBatch(apiFilters)
      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} redemption codes', { count })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteFilteredConfirm(false)
      }
    } finally {
      setIsDeletingFiltered(false)
    }
  }

  const deleteMatchingLabel = hasFilterConditions
    ? t('Delete matching redemption codes')
    : t('Delete all redemption codes')

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Redemption Codes Found')}
        emptyDescription={t(
          'No redemption codes available. Create your first redemption code to get started.'
        )}
        skeletonKeyPrefix='redemptions-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Filter by name or ID...'),
          searchDebounceMs: 500,
          onReset: () => {
            resetNameFilterInput()
            resetQuotaFilterInput()
          },
          additionalSearch: (
            <>
              <Input
                placeholder={t('Filter by name...')}
                value={nameFilterInput}
                onChange={onNameFilterInputChange}
                onCompositionStart={onNameFilterCompositionStart}
                onCompositionEnd={onNameFilterCompositionEnd}
                className='w-full sm:w-[150px] lg:w-[180px]'
              />
              <Input
                type='number'
                inputMode='decimal'
                min='0'
                step='any'
                placeholder={t('Filter by quota...')}
                value={quotaFilterInput}
                onChange={onQuotaFilterInputChange}
                onCompositionStart={onQuotaFilterCompositionStart}
                onCompositionEnd={onQuotaFilterCompositionEnd}
                aria-invalid={hasInvalidQuotaFilter || undefined}
                className='w-full sm:w-[130px] lg:w-[160px]'
              />
            </>
          ),
          hasAdditionalFilters: Boolean(
            nameFilter.trim() || quotaFilter.trim()
          ),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: redemptionStatusOptions,
              singleSelect: true,
            },
          ],
          preActions: (
            <>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant='outline'
                      size='icon'
                      onClick={handleExportMatching}
                      disabled={isExportingFiltered || hasInvalidQuotaFilter}
                      className='size-8'
                      aria-label={t('Download matching redemption codes')}
                      title={t('Download matching redemption codes')}
                    />
                  }
                >
                  <Download />
                  <span className='sr-only'>
                    {t('Download matching redemption codes')}
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{t('Download matching redemption codes')}</p>
                </TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant='destructive'
                      size='icon'
                      onClick={() => setShowDeleteFilteredConfirm(true)}
                      disabled={isDeletingFiltered || hasInvalidQuotaFilter}
                      className='size-8'
                      aria-label={deleteMatchingLabel}
                      title={deleteMatchingLabel}
                    />
                  }
                >
                  <Trash2 />
                  <span className='sr-only'>{deleteMatchingLabel}</span>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{deleteMatchingLabel}</p>
                </TooltipContent>
              </Tooltip>
            </>
          ),
        }}
        getRowClassName={(row, { isMobile }) =>
          isDisabledRedemptionRow(row.original)
            ? isMobile
              ? DISABLED_ROW_MOBILE
              : DISABLED_ROW_DESKTOP
            : undefined
        }
        bulkActions={<DataTableBulkActions table={table} />}
      />

      <ConfirmDialog
        destructive
        open={showDeleteFilteredConfirm}
        onOpenChange={setShowDeleteFilteredConfirm}
        handleConfirm={handleDeleteMatching}
        isLoading={isDeletingFiltered}
        className='max-w-md'
        title={
          hasFilterConditions
            ? t('Delete Matching Redemption Codes?')
            : t('Delete All Redemption Codes?')
        }
        desc={
          <>
            {hasFilterConditions
              ? t(
                  'This will delete redemption codes matching the current filters.'
                )
              : t('This will delete all redemption codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={deleteMatchingLabel}
      />
    </>
  )
}
