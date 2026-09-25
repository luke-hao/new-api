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
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type {
  SortingState,
  Table,
  VisibilityState,
} from '@tanstack/react-table'
import { Activity, KeyRound, RefreshCw, Rows3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getUserGroups } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DataTablePage,
  DataTableToolbar,
  useDataTable,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { getApiKeys, getTokenMetrics, searchApiKeys } from '../api'
import { API_KEY_STATUS_OPTIONS, API_KEY_STATUSES } from '../constants'
import type { ApiKey, TokenListFilters, TokenMetric } from '../types'
import { ApiEndpointsPanel } from './api-endpoints-panel'
import {
  KeyQuotaCell,
  KeyTimeCell,
  KeyActivityCell,
} from './api-key-metrics-cells'
import {
  ApiKeyCell,
  ApiKeyGroupCell,
  ModelLimitsCell,
  IpRestrictionsCell,
} from './api-keys-cells'
import { useApiKeysColumns, useKeyGroups } from './api-keys-columns'
import { useApiKeys } from './api-keys-provider'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { DataTableRowActions } from './data-table-row-actions'

const route = getRouteApi('/_authenticated/keys/')
const EMPTY_KEYS: ApiKey[] = []
const EMPTY_METRICS: Record<number, TokenMetric> = {}

function MobileKeys(props: {
  table: Table<ApiKey>
  loading: boolean
  metrics: Record<number, TokenMetric>
  unit: 'quota' | 'tokens'
  consumptionStatus?: string
}) {
  const { t } = useTranslation()
  const groups = useKeyGroups()
  if (props.loading)
    return (
      <div className='space-y-3'>
        {[1, 2, 3].map((n) => (
          <Skeleton key={n} className='h-44 w-full rounded-xl' />
        ))}
      </div>
    )
  if (!props.table.getRowModel().rows.length)
    return (
      <div className='text-muted-foreground rounded-xl border border-dashed p-10 text-center'>
        {t('No API Keys Found')}
      </div>
    )
  return (
    <div className='space-y-3'>
      <div className='flex items-center gap-2 text-xs'>
        <Checkbox
          checked={props.table.getIsAllPageRowsSelected()}
          indeterminate={props.table.getIsSomePageRowsSelected()}
          onCheckedChange={(v) => props.table.toggleAllPageRowsSelected(!!v)}
          aria-label={t('Select all')}
        />
        {t('Select all')}
      </div>
      {props.table.getRowModel().rows.map((row) => {
        const key = row.original
        const status = API_KEY_STATUSES[key.status]
        return (
          <article
            key={row.id}
            className='bg-card min-w-0 space-y-3 rounded-xl border p-3.5'
          >
            <div className='flex items-start justify-between gap-2'>
              <div className='flex min-w-0 items-center gap-2'>
                <Checkbox
                  checked={row.getIsSelected()}
                  onCheckedChange={(v) => row.toggleSelected(!!v)}
                  aria-label={t('Select row')}
                />
                <span className='font-medium break-all'>{key.name}</span>
              </div>
              {status && (
                <StatusBadge
                  label={t(status.label)}
                  variant={status.variant}
                  copyable={false}
                />
              )}
            </div>
            <div className='flex items-center justify-between gap-1'>
              <ApiKeyCell apiKey={key} />
              <DataTableRowActions row={row} />
            </div>
            <div className='bg-muted/30 grid grid-cols-[minmax(0,1fr)_auto] gap-5 rounded-lg p-3'>
              <KeyQuotaCell
                apiKey={key}
                metric={props.metrics[key.id]}
                unit={props.unit}
                consumptionStatus={props.consumptionStatus}
              />
              <KeyActivityCell metric={props.metrics[key.id]} />
            </div>
            <ApiKeyGroupCell apiKey={key} options={groups} />
            <details className='text-xs'>
              <summary className='text-muted-foreground cursor-pointer py-1'>
                {t('Limits & timestamps')}
              </summary>
              <div className='grid gap-3 pt-3'>
                <div className='flex gap-4'>
                  <ModelLimitsCell apiKey={key} />
                  <IpRestrictionsCell apiKey={key} />
                </div>
                <KeyTimeCell apiKey={key} />
              </div>
            </details>
          </article>
        )
      })}
      <DataTableBulkActions table={props.table} />
    </div>
  )
}

export function ApiKeysTable() {
  const { t } = useTranslation()
  const { refreshTrigger, triggerRefresh, setOpen } = useApiKeys()
  const [sorting, setSorting] = useState<SortingState>([])
  const [compact, setCompact] = useState(false)
  const [unit, setUnit] = useState<'quota' | 'tokens'>('quota')
  const [visibility, setVisibility] = useState<VisibilityState>({
    status: false,
    created_time: false,
    expired_time: false,
    used_quota: false,
  })
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
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'group', searchKey: 'group', type: 'array' },
    ],
  })
  const [tokenFilterInput, setTokenFilterInput] = useState('')
  const [tokenFilter, setTokenFilter] = useState('')
  useEffect(() => {
    const timer = setTimeout(() => setTokenFilter(tokenFilterInput.trim()), 350)
    return () => clearTimeout(timer)
  }, [tokenFilterInput])
  const statusFilter = columnFilters.find((f) => f.id === 'status')?.value as
    | string[]
    | undefined
  const groupFilter = columnFilters.find((f) => f.id === 'group')?.value as
    | string[]
    | undefined
  const filters: TokenListFilters = {
    status: statusFilter?.join(','),
    group: groupFilter?.[0],
    sort: sorting[0]?.id,
    order: sorting[0]?.desc ? 'desc' : 'asc',
    name_match: 'contains',
  }
  const shouldSearch = Boolean(globalFilter?.trim() || tokenFilter.trim())
  const { data, isLoading, isFetching, error } = useQuery({
    queryKey: [
      'keys',
      pagination.pageIndex,
      pagination.pageSize,
      globalFilter,
      tokenFilter,
      filters,
      refreshTrigger,
      shouldSearch,
    ],
    queryFn: async () => {
      const params = {
        p: pagination.pageIndex + 1,
        size: pagination.pageSize,
        ...filters,
      }
      const response = shouldSearch
        ? await searchApiKeys({
            ...params,
            keyword: globalFilter,
            token: tokenFilter,
          })
        : await getApiKeys(params)
      if (!response.success)
        throw new Error(response.message || 'Failed to load API keys')
      return response.data
    },
    placeholderData: (previous) => previous,
  })
  const keys = data?.items || EMPTY_KEYS
  const ids = useMemo(() => keys.map((key) => key.id), [keys])
  const metricsQuery = useQuery({
    queryKey: ['token-metrics', ids, refreshTrigger],
    queryFn: () => getTokenMetrics(ids),
    enabled: ids.length > 0,
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
    retry: 1,
  })
  const metrics = useMemo(
    () =>
      metricsQuery.data
        ? Object.fromEntries(
            metricsQuery.data.items.map((item) => [item.id, item])
          )
        : EMPTY_METRICS,
    [metricsQuery.data]
  )
  const columns = useApiKeysColumns(
    metrics,
    unit,
    metricsQuery.data?.consumption_status
  )
  const groups = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: 30000,
  })
  const { table } = useDataTable({
    data: keys,
    columns,
    enableRowSelection: true,
    getRowId: (key) => String(key.id),
    columnFilters,
    globalFilter,
    pagination,
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    sorting,
    onSortingChange: (updater) => {
      setSorting(updater)
      onPaginationChange({ ...pagination, pageIndex: 0 })
    },
    manualPagination: true,
    manualFiltering: true,
    manualSorting: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
    columnVisibility: visibility,
    onColumnVisibilityChange: setVisibility,
  })
  const active = Object.values(metrics).reduce(
    (sum, metric) => sum + metric.active,
    0
  )
  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No API Keys Found')}
      emptyDescription={t('Try another filter or create a new API key.')}
      emptyAction={
        <Button onClick={() => setOpen('create')}>{t('Create API Key')}</Button>
      }
      applyHeaderSize
      className='keys-workspace'
      tableClassName={cn(
        'rounded-xl bg-card shadow-xs [&_td]:align-middle',
        compact ? '[&_td]:py-2' : '[&_td]:py-4'
      )}
      tableHeaderClassName='bg-muted/45'
      toolbar={
        <div className='space-y-5 pb-1'>
          <ApiEndpointsPanel />
          <div className='flex flex-wrap items-center justify-between gap-2 border-t pt-4'>
            <div className='text-muted-foreground flex flex-wrap items-center gap-4 text-xs'>
              <span className='inline-flex items-center gap-1.5'>
                <KeyRound className='size-3.5' />
                {t('Matching keys')}:{' '}
                <strong className='text-foreground'>
                  {data?.total ?? '—'}
                </strong>
              </span>
              <span className='inline-flex items-center gap-1.5'>
                <Activity className='size-3.5' />
                {t('Active on this page')}:{' '}
                <strong className='text-foreground'>
                  {metricsQuery.data ? active : '—'}
                </strong>
              </span>
              <span>
                {metricsQuery.data
                  ? t('Updated at') +
                    ' ' +
                    new Date(
                      metricsQuery.data.as_of * 1000
                    ).toLocaleTimeString()
                  : t('Live updates every 5 seconds')}
              </span>
            </div>
            <div className='flex items-center gap-1'>
              <Button
                variant='ghost'
                size='sm'
                onClick={() => setUnit(unit === 'quota' ? 'tokens' : 'quota')}
              >
                {t(unit === 'quota' ? 'Today: amount' : 'Today: tokens')}
              </Button>
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('Compact rows')}
                aria-pressed={compact}
                onClick={() => setCompact(!compact)}
              >
                <Rows3 className='size-4' />
              </Button>
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('Refresh')}
                disabled={isFetching}
                onClick={triggerRefresh}
              >
                <RefreshCw
                  className={cn('size-4', isFetching && 'animate-spin')}
                />
              </Button>
            </div>
          </div>
          <DataTableToolbar
            table={table}
            searchPlaceholder={t('Filter by name...')}
            searchDebounceMs={350}
            additionalSearch={
              <Input
                placeholder={t('Filter by API key...')}
                aria-label={t('Filter by API key...')}
                value={tokenFilterInput}
                onChange={(e) => {
                  setTokenFilterInput(e.target.value)
                  onPaginationChange({ ...pagination, pageIndex: 0 })
                }}
                className='w-full sm:w-52'
              />
            }
            hasAdditionalFilters={Boolean(tokenFilter || sorting.length)}
            onReset={() => {
              setTokenFilterInput('')
              setSorting([])
              table.resetRowSelection()
            }}
            filters={[
              {
                columnId: 'status',
                title: t('Status'),
                options: API_KEY_STATUS_OPTIONS,
                singleSelect: true,
              },
              {
                columnId: 'group',
                title: t('Group'),
                options: Object.keys(groups.data?.data || {}).map((group) => ({
                  label: group,
                  value: group,
                })),
                singleSelect: true,
              },
            ]}
          />
          {error && (
            <p role='alert' className='text-destructive text-sm'>
              {t('Failed to load API keys')} · {error.message}
            </p>
          )}
          {(metricsQuery.isError ||
            metricsQuery.data?.consumption_status === 'unavailable') && (
            <p role='status' className='text-destructive text-xs'>
              {t('Usage statistics temporarily unavailable. Refresh to retry.')}
            </p>
          )}
          {metricsQuery.data?.consumption_status === 'disabled' && (
            <p className='text-muted-foreground text-xs'>
              {t('Consumption logging is disabled')}
            </p>
          )}
        </div>
      }
      mobile={
        <MobileKeys
          table={table}
          loading={isLoading}
          metrics={metrics}
          unit={unit}
          consumptionStatus={metricsQuery.data?.consumption_status}
        />
      }
      getRowClassName={(row) =>
        row.original.status === 1 ? undefined : 'bg-muted/20'
      }
      bulkActions={<DataTableBulkActions table={table} />}
    />
  )
}
