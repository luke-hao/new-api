import { useCallback, useEffect, useRef, useState } from 'react'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { InfinityIcon, LoaderCircle, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { isSidebarModuleEnabled } from '@/lib/nav-modules'
import { Button } from '@/components/ui/button'
import { Main } from '@/components/layout'

type TicketResponse = {
  success: boolean
  message?: string
  data?: {
    ticket: string
    expires_at: number
  }
}

export const Route = createFileRoute('/_authenticated/canvas/')({
  beforeLoad: () => {
    if (!isSidebarModuleEnabled('chat', 'canvas')) {
      throw redirect({ to: '/dashboard' })
    }
  },
  component: CanvasPage,
})

function CanvasPage() {
  const { t } = useTranslation()
  const [iframeSrc, setIframeSrc] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const openingRef = useRef(false)
  const lastOpenedAtRef = useRef(0)

  const openCanvas = useCallback(async () => {
    const now = Date.now()
    if (openingRef.current || now - lastOpenedAtRef.current < 1500) return
    openingRef.current = true
    lastOpenedAtRef.current = now
    setError('')
    setLoading(true)
    setIframeSrc('')
    try {
      const response = await api.post<TicketResponse>(
        '/api/user/canvas/sso-ticket'
      )
      const payload = response.data
      if (!payload.success || !payload.data?.ticket) {
        throw new Error(payload.message || t('Failed to open infinite canvas'))
      }
      setIframeSrc(
        `/canvas-app/sso#ticket=${encodeURIComponent(payload.data.ticket)}`
      )
    } catch (requestError) {
      setLoading(false)
      setError(
        requestError instanceof Error
          ? requestError.message
          : t('Failed to open infinite canvas')
      )
    } finally {
      openingRef.current = false
    }
  }, [t])

  useEffect(() => {
    void openCanvas()
  }, [openCanvas])

  return (
    <Main className='bg-muted/20 relative p-0'>
      {iframeSrc ? (
        <iframe
          className='bg-background h-full min-h-0 w-full flex-1 border-0'
          src={iframeSrc}
          title={t('Infinite Canvas')}
          allow='clipboard-read; clipboard-write; fullscreen'
          onLoad={() => setLoading(false)}
        />
      ) : null}

      {loading ? (
        <div className='bg-background/90 absolute inset-0 flex items-center justify-center'>
          <div className='text-muted-foreground flex items-center gap-3 text-sm'>
            <LoaderCircle className='size-5 animate-spin' />
            {t('Opening infinite canvas...')}
          </div>
        </div>
      ) : null}

      {error ? (
        <div className='bg-background absolute inset-0 flex items-center justify-center p-6'>
          <div className='flex max-w-md flex-col items-center gap-4 text-center'>
            <InfinityIcon className='text-muted-foreground size-10' />
            <div>
              <h2 className='text-lg font-semibold'>{t('Infinite Canvas')}</h2>
              <p className='text-muted-foreground mt-1 text-sm'>{error}</p>
            </div>
            <Button onClick={() => void openCanvas()} disabled={loading}>
              <RefreshCw className='size-4' />
              {t('Retry')}
            </Button>
          </div>
        </div>
      ) : null}
    </Main>
  )
}
