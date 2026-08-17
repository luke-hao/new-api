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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { getUserGroups } from '@/features/users/api'
import { safeJsonParse } from '../utils/json-parser'

type UserGroupTableProps = {
  userGroups: string
  groupRatio: string
  onChange: (value: string) => void
}

type UserGroupRow = {
  _id: string
  name: string
  originalName: string | null
  description: string
  userCount: number
  billingGroup: boolean
}

const sectionCardClassName =
  'relative shadow-sm ring-0 before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:border before:border-border/90'
const sectionHeaderClassName = 'border-b bg-muted/20'

let userGroupRowId = 0
function createRowId() {
  userGroupRowId += 1
  return `ugr_${userGroupRowId}`
}

function serializeRows(rows: UserGroupRow[]) {
  const groups: Record<string, string> = {}
  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groups[name] = row.description
  }
  return JSON.stringify(groups, null, 2)
}

function rowsSignature(rows: UserGroupRow[]) {
  return JSON.stringify(
    safeJsonParse<Record<string, string>>(serializeRows(rows), {
      fallback: {},
      silent: true,
    })
  )
}

function sourceSignature(userGroups: string) {
  return JSON.stringify(
    safeJsonParse<Record<string, string>>(userGroups, {
      fallback: {},
      silent: true,
    })
  )
}

export function UserGroupTable({
  userGroups,
  groupRatio,
  onChange,
}: UserGroupTableProps) {
  const { t } = useTranslation()
  const { data: usageResponse } = useQuery({
    queryKey: ['user-identity-groups'],
    queryFn: getUserGroups,
    staleTime: 60 * 1000,
  })

  const usageMap = useMemo(
    () =>
      new Map(
        (usageResponse?.data ?? []).map((group) => [group.name, group] as const)
      ),
    [usageResponse?.data]
  )
  const billingGroups = useMemo(
    () =>
      new Set(
        Object.keys(
          safeJsonParse<Record<string, number>>(groupRatio, {
            fallback: {},
            silent: true,
          })
        )
      ),
    [groupRatio]
  )

  const buildRows = useCallback(() => {
    const groups = safeJsonParse<Record<string, string>>(userGroups, {
      fallback: {},
      context: 'user identity groups',
    })
    return Object.entries(groups).map(([name, description]) => ({
      _id: createRowId(),
      name,
      originalName: name,
      description,
      userCount: usageMap.get(name)?.user_count ?? 0,
      billingGroup: billingGroups.has(name),
    }))
  }, [billingGroups, usageMap, userGroups])

  const [rows, setRows] = useState<UserGroupRow[]>(buildRows)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setRows((currentRows) => {
      if (rowsSignature(currentRows) !== sourceSignature(userGroups)) {
        return buildRows()
      }
      return currentRows.map((row) => ({
        ...row,
        userCount: usageMap.get(row.name)?.user_count ?? 0,
        billingGroup: billingGroups.has(row.name),
      }))
    })
  }, [billingGroups, buildRows, usageMap, userGroups])

  const emitRows = useCallback(
    (nextRows: UserGroupRow[]) => {
      setRows(nextRows)
      onChange(serializeRows(nextRows))
    },
    [onChange]
  )

  const updateRow = useCallback(
    (id: string, field: 'name' | 'description', value: string) => {
      emitRows(
        rows.map((row) => (row._id === id ? { ...row, [field]: value } : row))
      )
    },
    [emitRows, rows]
  )

  const addRow = useCallback(() => {
    const names = new Set(rows.map((row) => row.name))
    let index = 1
    let name = `user_group_${index}`
    while (names.has(name)) {
      index += 1
      name = `user_group_${index}`
    }
    emitRows([
      ...rows,
      {
        _id: createRowId(),
        name,
        originalName: null,
        description: '',
        userCount: 0,
        billingGroup: billingGroups.has(name),
      },
    ])
  }, [billingGroups, emitRows, rows])

  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const row of rows) {
      const name = row.name.trim()
      if (!name) continue
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return Array.from(counts.entries())
      .filter(([, count]) => count > 1)
      .map(([name]) => name)
  }, [rows])

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('User identity groups')}</CardTitle>
            <CardDescription>
              {t(
                'Groups assigned to user accounts by administrators. They are separate from billing and token groups.'
              )}
            </CardDescription>
          </div>
          <Button onClick={addRow} size='sm' className='sm:self-start'>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add user group')}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <StaticDataTable
            data={rows}
            getRowKey={(row) => row._id}
            emptyClassName='text-muted-foreground h-20 text-sm'
            emptyContent={t('No user identity groups configured.')}
            columns={[
              {
                id: 'name',
                header: t('User group name'),
                className: 'min-w-44',
                cell: (row) => (
                  <Input
                    value={row.name}
                    disabled={row.originalName !== null}
                    aria-invalid={duplicateNames.includes(row.name.trim())}
                    onChange={(event) =>
                      updateRow(row._id, 'name', event.target.value)
                    }
                  />
                ),
              },
              {
                id: 'description',
                header: t('Description'),
                className: 'min-w-64',
                cell: (row) => (
                  <Input
                    value={row.description}
                    placeholder={t('User group description')}
                    onChange={(event) =>
                      updateRow(row._id, 'description', event.target.value)
                    }
                  />
                ),
              },
              {
                id: 'users',
                header: t('Users'),
                className: 'w-24 text-center',
                cellClassName: 'text-center',
                cell: (row) => row.userCount,
              },
              {
                id: 'role',
                header: t('Group role'),
                className: 'w-36',
                cell: (row) =>
                  row.billingGroup ? (
                    <StatusBadge variant='info' copyable={false}>
                      {t('Also a billing group')}
                    </StatusBadge>
                  ) : (
                    <StatusBadge variant='neutral' copyable={false}>
                      {t('User group only')}
                    </StatusBadge>
                  ),
              },
              {
                id: 'actions',
                header: t('Actions'),
                className: 'w-16 text-right',
                cellClassName: 'text-right',
                cell: (row) => (
                  <Button
                    variant='ghost'
                    size='sm'
                    disabled={row.name === 'default'}
                    onClick={() =>
                      emitRows(rows.filter((item) => item._id !== row._id))
                    }
                    aria-label={t('Delete')}
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                ),
              },
            ]}
          />

          {duplicateNames.length > 0 && (
            <p className='text-destructive text-sm'>
              {t('Duplicate group names: {{names}}', {
                names: duplicateNames.join(', '),
              })}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
