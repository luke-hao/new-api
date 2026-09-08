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
import { Building2, Check, Copy, QrCode } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const ENTERPRISE_QQ_GROUP = '1047030984'
// Keep the QQ-issued image intact; a group number is not its invite payload.
const ENTERPRISE_QQ_GROUP_QR_IMAGE = '/images/qq-group-1047030984.webp'

type EnterpriseContactButtonProps = {
  className?: string
  appearance?: 'dark' | 'light'
}

export function EnterpriseContactButton({
  className,
  appearance = 'dark',
}: EnterpriseContactButtonProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(ENTERPRISE_QQ_GROUP)
      setCopied(true)
      toast.success(t('home.enterprise.copied'))
      window.setTimeout(() => setCopied(false), 2200)
    } catch {
      toast.info(t('home.enterprise.copyFallback'))
    }
  }

  return (
    <div className={cn('flex flex-wrap items-center gap-3', className)}>
      <Button
        type='button'
        variant='outline'
        className={cn(
          'h-11 rounded-lg px-5 text-sm font-semibold',
          appearance === 'dark'
            ? 'border-white/20 bg-white/[0.04] text-white hover:bg-white/[0.09] hover:text-white'
            : 'border-[#cbd8e8] bg-white text-[#11213a] hover:bg-[#eef4fb]'
        )}
        onClick={() => setOpen(true)}
      >
        <Building2 className='size-4' aria-hidden='true' />
        {t('home.enterprise.action')}
      </Button>
      <button
        type='button'
        onClick={handleCopy}
        className={cn(
          'inline-flex h-9 items-center gap-2 rounded-lg border px-3 font-mono text-xs font-semibold',
          appearance === 'dark'
            ? 'border-cyan-300/25 bg-cyan-300/[0.08] text-cyan-100'
            : 'border-[#b9cce3] bg-[#eef6ff] text-[#174a82]'
        )}
        aria-label={t('home.enterprise.copyAria', {
          number: ENTERPRISE_QQ_GROUP,
        })}
      >
        QQ {ENTERPRISE_QQ_GROUP}
        {copied ? (
          <Check className='size-3.5' aria-hidden='true' />
        ) : (
          <Copy className='size-3.5' aria-hidden='true' />
        )}
      </button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-md'>
          <DialogHeader className='pr-8'>
            <DialogTitle className='flex items-center gap-2'>
              <QrCode className='size-4 text-cyan-500' aria-hidden='true' />
              {t('home.enterprise.qrTitle')}
            </DialogTitle>
            <DialogDescription>
              {t('home.enterprise.qrDescription')}
            </DialogDescription>
          </DialogHeader>

          <div className='flex flex-col items-center gap-4 py-2'>
            <img
              src={ENTERPRISE_QQ_GROUP_QR_IMAGE}
              alt={t('home.enterprise.qrAlt', { number: ENTERPRISE_QQ_GROUP })}
              width={1080}
              height={1920}
              className='max-h-[55dvh] w-auto max-w-full rounded-xl'
            />
            <button
              type='button'
              onClick={handleCopy}
              className='border-border bg-muted/40 hover:bg-muted inline-flex min-h-10 items-center gap-2 rounded-lg border px-4 py-2 font-mono text-sm font-semibold'
              aria-label={t('home.enterprise.copyAria', {
                number: ENTERPRISE_QQ_GROUP,
              })}
            >
              QQ {ENTERPRISE_QQ_GROUP}
              {copied ? (
                <Check className='size-4 text-emerald-500' aria-hidden='true' />
              ) : (
                <Copy className='size-4' aria-hidden='true' />
              )}
            </button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
