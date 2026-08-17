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
import { useCallback, useEffect, useState } from 'react'
import { getAffiliateRebates } from '../api'
import type { AffiliateRebateRecord } from '../types'

const PAGE_SIZE = 10

export function useAffiliateRebates(open: boolean) {
  const [records, setRecords] = useState<AffiliateRebateRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)

  const fetchRecords = useCallback(async () => {
    if (!open) return
    setLoading(true)
    setError(false)
    try {
      const response = await getAffiliateRebates(page, PAGE_SIZE)
      if (!response.success || !response.data) {
        setError(true)
        return
      }
      setRecords(response.data.items || [])
      setTotal(response.data.total || 0)
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [open, page])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void fetchRecords()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [fetchRecords])

  return {
    records,
    total,
    page,
    pageSize: PAGE_SIZE,
    loading,
    error,
    setPage,
    refetch: fetchRecords,
  }
}
