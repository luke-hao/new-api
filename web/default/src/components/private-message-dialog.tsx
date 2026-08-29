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
import { lazy, Suspense, useEffect, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  useAuthStore,
  type AuthUser,
  type PrivateMessage,
} from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const LazyMarkdown = lazy(() =>
  import('@/components/ui/markdown').then(({ Markdown }) => ({
    default: Markdown,
  }))
)

const PRIVATE_MESSAGE_POLL_INTERVAL_MS = 30_000

function getSettingPrivateMessage(setting: unknown): PrivateMessage | null {
  if (!setting) return null

  if (typeof setting === 'object') {
    const message = (setting as Record<string, unknown>).private_message
    return isPrivateMessage(message) ? message : null
  }

  if (typeof setting !== 'string') return null

  try {
    const parsed = JSON.parse(setting) as Record<string, unknown>
    return isPrivateMessage(parsed.private_message)
      ? parsed.private_message
      : null
  } catch {
    return null
  }
}

function isPrivateMessage(value: unknown): value is PrivateMessage {
  if (!value || typeof value !== 'object') return false
  const content = (value as Record<string, unknown>).content
  return typeof content === 'string' && content.trim().length > 0
}

function hashString(input: string): string {
  let hash = 0
  for (let i = 0; i < input.length; i += 1) {
    hash = (hash << 5) - hash + input.charCodeAt(i)
    hash |= 0
  }
  return hash.toString(36)
}

function getMessageKey(userId: number, message: PrivateMessage): string {
  const id =
    message.id ||
    hashString(
      JSON.stringify({
        created_at: message.created_at || 0,
        title: message.title || '',
        content: message.content || '',
      })
    )
  return `private-message-read:${userId}:${id}`
}

function isUnreadMessage(storageKey: string): boolean {
  try {
    return window.localStorage.getItem(storageKey) !== 'true'
  } catch {
    return true
  }
}

interface PrivateMessageContentProps {
  message: PrivateMessage
  storageKey: string
}

function PrivateMessageContent({
  message,
  storageKey,
}: PrivateMessageContentProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(() => isUnreadMessage(storageKey))

  const close = () => {
    try {
      window.localStorage.setItem(storageKey, 'true')
    } catch {
      /* empty */
    }
    setOpen(false)
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => nextOpen && setOpen(true)}>
      <DialogContent
        showCloseButton={false}
        className='max-h-[calc(100svh-2rem)] gap-0 overflow-hidden p-0 sm:max-w-2xl'
      >
        <div className='bg-destructive text-destructive-foreground flex items-center gap-3 px-5 py-4'>
          <div className='bg-destructive-foreground/15 flex size-10 shrink-0 items-center justify-center rounded-full'>
            <AlertTriangle className='size-5' />
          </div>
          <DialogHeader className='gap-1'>
            <DialogTitle className='text-xl leading-tight font-semibold'>
              {message.title || t('Important notice')}
            </DialogTitle>
          </DialogHeader>
        </div>

        <div className='max-h-[min(58svh,34rem)] overflow-y-auto px-5 py-5'>
          <Suspense
            fallback={<div className='bg-muted/40 min-h-32 animate-pulse' />}
          >
            <LazyMarkdown className='prose-base prose-p:text-base prose-p:leading-7 prose-li:text-base'>
              {message.content || ''}
            </LazyMarkdown>
          </Suspense>
        </div>

        <DialogFooter className='m-0 rounded-none'>
          <Button size='lg' onClick={close} className='w-full sm:w-auto'>
            {t('I understand')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function PrivateMessageDialog() {
  const user = useAuthStore((state) => state.auth.user)
  const setUser = useAuthStore((state) => state.auth.setUser)

  const message = user
    ? user.private_message || getSettingPrivateMessage(user.setting)
    : null
  const storageKey = user?.id && message ? getMessageKey(user.id, message) : ''

  useEffect(() => {
    if (!user?.id) return

    let cancelled = false
    const refreshSelf = async () => {
      if (document.visibilityState === 'hidden') return

      const response = await getSelf().catch(() => null)
      if (!cancelled && response?.success && response.data) {
        setUser(response.data as AuthUser)
      }
    }

    const timer = window.setInterval(
      refreshSelf,
      PRIVATE_MESSAGE_POLL_INTERVAL_MS
    )
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [setUser, user?.id])

  if (!message || !storageKey) return null

  return (
    <PrivateMessageContent
      key={storageKey}
      message={message}
      storageKey={storageKey}
    />
  )
}
