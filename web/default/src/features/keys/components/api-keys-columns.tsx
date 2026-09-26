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
import { useQuery } from '@tanstack/react-query'
import type { CellContext, ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { getUserGroups } from '@/lib/api'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Checkbox } from '@/components/ui/checkbox'
import { StatusBadge } from '@/components/status-badge'
import { API_KEY_STATUSES } from '../constants'
import type { ApiKey, TokenMetric } from '../types'
import type { ApiKeyGroupOption } from './api-key-group-combobox'
import {
  KeyQuotaCell,
  KeyActivityCell,
  KeyTimeCell,
} from './api-key-metrics-cells'
import {
  ApiKeyCell,
  ApiKeyGroupCell,
  ModelLimitsCell,
  IpRestrictionsCell,
} from './api-keys-cells'
import { DataTableRowActions } from './data-table-row-actions'

export function useKeyGroups() {
  const { data } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: 30000,
  })
  const options: ApiKeyGroupOption[] = Object.entries(data?.data || {}).map(
    ([group, info]) => ({
      value: group,
      label: group,
      desc: info.desc || group,
      ratio: info.ratio,
      auto_eligible: info.auto_eligible,
      auto_types: info.auto_types,
    })
  )
  return options
}
// Keep the component type stable when live metrics recreate column definitions.
// flexRender treats an inline cell function as a React component: a new function
// would unmount its open popover and personal Auto draft on every polling tick.
// eslint-disable-next-line react-refresh/only-export-components -- Stable TanStack renderer for this columns hook.
function GroupTableCell(props: CellContext<ApiKey, unknown>) {
  const options = useKeyGroups()
  return (
    <ApiKeyGroupCell
      key={props.row.original.id + ':' + (props.row.original.group || '')}
      apiKey={props.row.original}
      options={options}
    />
  )
}

export function useApiKeysColumns(
  metrics: Record<number, TokenMetric>,
  unit: 'quota' | 'tokens',
  consumptionStatus?: string
): ColumnDef<ApiKey>[] {
  const { t } = useTranslation()
  return [
    {
      id: 'select',
      size: 36,
      enableSorting: false,
      enableHiding: false,
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('Select all')}
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('Select row')}
        />
      ),
    },
    {
      accessorKey: 'name',
      header: t('Name / API Key'),
      size: 235,
      enableHiding: false,
      cell: ({ row }) => {
        const config = API_KEY_STATUSES[row.original.status]
        return (
          <div className='min-w-0 space-y-1.5'>
            <div className='flex flex-wrap items-center gap-x-2 gap-y-1'>
              <span className='font-medium break-all'>{row.original.name}</span>
              {config && (
                <StatusBadge
                  label={t(config.label)}
                  variant={config.variant}
                  copyable={false}
                />
              )}
            </div>
            <ApiKeyCell apiKey={row.original} />
          </div>
        )
      },
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      enableHiding: true,
      size: 90,
      cell: ({ row }) => {
        const config = API_KEY_STATUSES[row.original.status]
        return config ? (
          <StatusBadge
            label={t(config.label)}
            variant={config.variant}
            copyable={false}
          />
        ) : null
      },
    },
    {
      id: 'remain_quota',
      accessorKey: 'remain_quota',
      header: t('Quota & usage'),
      size: 174,
      cell: ({ row }) => (
        <KeyQuotaCell
          apiKey={row.original}
          metric={metrics[row.original.id]}
          unit={unit}
          consumptionStatus={consumptionStatus}
        />
      ),
    },
    {
      accessorKey: 'group',
      header: t('Group'),
      size: 234,
      cell: GroupTableCell,
    },
    {
      id: 'activity',
      header: t('Live activity'),
      size: 112,
      enableSorting: false,
      cell: ({ row }) => <KeyActivityCell metric={metrics[row.original.id]} />,
    },
    {
      id: 'restrictions',
      header: t('Models / IP'),
      size: 128,
      enableSorting: false,
      cell: ({ row }) => (
        <div className='space-y-1.5 text-xs'>
          <div>
            <span className='text-muted-foreground mr-1'>{t('Models')}</span>
            <ModelLimitsCell apiKey={row.original} />
          </div>
          <div>
            <span className='text-muted-foreground mr-1'>IP</span>
            <IpRestrictionsCell apiKey={row.original} />
          </div>
        </div>
      ),
    },
    {
      accessorKey: 'accessed_time',
      header: t('Time'),
      size: 162,
      cell: ({ row }) => <KeyTimeCell apiKey={row.original} />,
    },
    {
      accessorKey: 'created_time',
      header: t('Created'),
      size: 155,
      cell: ({ row }) => (
        <span className='text-xs'>
          {formatTimestampToDate(row.original.created_time)}
        </span>
      ),
    },
    {
      accessorKey: 'expired_time',
      header: t('Expires'),
      size: 155,
      cell: ({ row }) => (
        <span className='text-xs'>
          {row.original.expired_time === -1
            ? t('Never')
            : formatTimestampToDate(row.original.expired_time)}
        </span>
      ),
    },
    {
      accessorKey: 'used_quota',
      header: t('Total used'),
      size: 120,
      cell: ({ row }) => formatQuota(row.original.used_quota),
    },
    {
      id: 'actions',
      header: t('Actions'),
      enableSorting: false,
      enableHiding: false,
      size: 110,
      meta: { pinned: 'right' as const },
      cell: ({ row }) => <DataTableRowActions row={row} />,
    },
  ]
}
