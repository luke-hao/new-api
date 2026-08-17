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
import { useState } from 'react'
import { Megaphone, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  broadcastPrivateMessage,
  clearBroadcastPrivateMessage,
} from '../api'

type UsersBroadcastPrivateMessageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const TITLE_MAX_LENGTH = 80
const CONTENT_MAX_LENGTH = 5000

export function UsersBroadcastPrivateMessageDialog({
  open,
  onOpenChange,
}: UsersBroadcastPrivateMessageDialogProps) {
  const { t } = useTranslation()
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [clearConfirmOpen, setClearConfirmOpen] = useState(false)

  const reset = () => {
    setTitle('')
    setContent('')
  }

  const close = () => {
    if (loading || clearing) return
    onOpenChange(false)
    reset()
  }

  const submit = async () => {
    const nextContent = content.trim()
    if (!nextContent) {
      toast.error(t('Private message content is required'))
      return
    }

    setLoading(true)
    try {
      const response = await broadcastPrivateMessage({
        title: title.trim(),
        content: nextContent,
      })

      if (response.success) {
        toast.success(
          t('Private message sent to {{count}} users', {
            count: response.data?.count ?? 0,
          })
        )
        onOpenChange(false)
        reset()
      } else {
        toast.error(response.message || t('Failed to send private message'))
      }
    } catch {
      toast.error(t('Failed to send private message'))
    } finally {
      setLoading(false)
    }
  }

  const clearCurrentBroadcast = async () => {
    setClearing(true)
    try {
      const response = await clearBroadcastPrivateMessage()

      if (response.success) {
        toast.success(
          t('Broadcast private message cleared for {{count}} users', {
            count: response.data?.count ?? 0,
          })
        )
        setClearConfirmOpen(false)
      } else {
        toast.error(
          response.message || t('Failed to clear private message broadcast')
        )
      }
    } catch {
      toast.error(t('Failed to clear private message broadcast'))
    } finally {
      setClearing(false)
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && close()}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Megaphone className='size-5' />
              {t('Broadcast Private Message')}
            </DialogTitle>
            <DialogDescription>
              {t('This will replace the private popup message for every user.')}
            </DialogDescription>
          </DialogHeader>

          <div className='grid gap-4'>
            <div className='grid gap-2'>
              <div className='flex items-center justify-between gap-3'>
                <Label htmlFor='broadcast-private-message-title'>
                  {t('Private Message Title')}
                </Label>
                <span className='text-muted-foreground text-xs tabular-nums'>
                  {title.length} / {TITLE_MAX_LENGTH}
                </span>
              </div>
              <Input
                id='broadcast-private-message-title'
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder={t('Important notice')}
                maxLength={TITLE_MAX_LENGTH}
              />
            </div>

            <div className='grid gap-2'>
              <div className='flex items-center justify-between gap-3'>
                <Label htmlFor='broadcast-private-message-content'>
                  {t('Private Message Content')}
                </Label>
                <span className='text-muted-foreground text-xs tabular-nums'>
                  {content.length} / {CONTENT_MAX_LENGTH}
                </span>
              </div>
              <Textarea
                id='broadcast-private-message-content'
                value={content}
                onChange={(event) => setContent(event.target.value)}
                placeholder={t(
                  'Supports Markdown. This is visible to the user.'
                )}
                rows={7}
                maxLength={CONTENT_MAX_LENGTH}
                className='h-40 max-h-40 resize-none overflow-y-auto [field-sizing:fixed]'
              />
            </div>
          </div>

          <DialogFooter className='sm:justify-between'>
            <Button
              type='button'
              variant='destructive'
              onClick={() => setClearConfirmOpen(true)}
              disabled={loading || clearing}
              className='gap-2'
            >
              <Trash2 className='size-4' />
              {t('Clear Broadcast')}
            </Button>
            <div className='flex flex-col-reverse gap-2 sm:flex-row'>
              <Button
                type='button'
                variant='outline'
                onClick={close}
                disabled={loading || clearing}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                onClick={submit}
                disabled={loading || clearing || !content.trim()}
              >
                {loading ? t('Sending...') : t('Send to All Users')}
              </Button>
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={clearConfirmOpen} onOpenChange={setClearConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Clear broadcast message?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This will stop the current global private popup message from being shown to new users and clear matching messages from existing users.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={clearing}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={clearCurrentBroadcast}
              disabled={clearing}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {clearing ? t('Clearing...') : t('Clear Broadcast')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
