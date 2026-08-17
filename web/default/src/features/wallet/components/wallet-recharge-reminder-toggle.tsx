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
import { Megaphone } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

interface WalletRechargeReminderToggleProps {
  checked: boolean
  disabled?: boolean
  onCheckedChange: (checked: boolean) => void
}

export function WalletRechargeReminderToggle({
  checked,
  disabled,
  onCheckedChange,
}: WalletRechargeReminderToggleProps) {
  const { t } = useTranslation()

  return (
    <div className='border-border/80 bg-card flex flex-col gap-3 rounded-lg border px-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-4'>
      <div className='flex min-w-0 items-start gap-3'>
        <div className='bg-muted mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg'>
          <Megaphone className='h-4 w-4' />
        </div>
        <div className='min-w-0 space-y-1'>
          <Label
            htmlFor='wallet-recharge-reminder-switch'
            className='text-sm font-medium'
          >
            {t('钱包充值提醒')}
          </Label>
          <p className='text-muted-foreground text-sm'>
            {t(
              '开启后，所有用户的钱包添加资金区域会替换为前往充值页面的提醒。'
            )}
          </p>
        </div>
      </div>
      <Switch
        id='wallet-recharge-reminder-switch'
        checked={checked}
        disabled={disabled}
        onCheckedChange={onCheckedChange}
        className='self-start sm:self-center'
      />
    </div>
  )
}
