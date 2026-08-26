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
import { Building2, Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

const ENTERPRISE_QQ_GROUP = '1023145006'

type EnterpriseContactButtonProps = {
  className?: string
  appearance?: 'dark' | 'light'
}

export function EnterpriseContactButton({
  className,
  appearance = 'dark',
}: EnterpriseContactButtonProps) {
  const { t } = useTranslation()
  const [revealed, setRevealed] = useState(false)
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    setRevealed(true)
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
        onClick={handleCopy}
      >
        <Building2 className='size-4' aria-hidden='true' />
        {t('home.enterprise.action')}
      </Button>
      {revealed ? (
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
      ) : null}
    </div>
  )
}
