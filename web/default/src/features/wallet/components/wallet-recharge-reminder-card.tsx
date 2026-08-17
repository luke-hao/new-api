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
import { Link } from '@tanstack/react-router'
import { AlertTriangle, ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'

export function WalletRechargeReminderCard() {
  const { t } = useTranslation()

  return (
    <TitledCard
      title={t('请前往充值页面完成充值')}
      description={t('当前钱包页充值入口已切换至充值页面。')}
      icon={<AlertTriangle className='h-4 w-4' />}
      iconClassName='bg-red-500/10 text-red-600 dark:text-red-400'
      disableHoverEffect
      contentClassName='space-y-4'
    >
      <Alert className='border-red-500/35 bg-red-500/10 text-red-950 dark:text-red-100'>
        <AlertTriangle className='h-4 w-4 text-red-600 dark:text-red-400' />
        <AlertTitle>{t('充值提醒')}</AlertTitle>
        <AlertDescription className='text-red-900/80 dark:text-red-100/80'>
          {t('请前往充值页面完成充值操作。')}
        </AlertDescription>
      </Alert>

      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <p className='text-muted-foreground text-sm'>
          {t('此提醒由管理员配置。')}
        </p>
        <Button
          size='lg'
          className='w-full gap-2 sm:w-auto'
          render={<Link to='/recharge' />}
        >
          {t('去充值')}
          <ArrowRight className='h-4 w-4' />
        </Button>
      </div>
    </TitledCard>
  )
}
