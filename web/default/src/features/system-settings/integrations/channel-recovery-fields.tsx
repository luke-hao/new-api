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
import type { Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  recoveryReasonLabels,
  type RecoveryPolicy,
} from '../utils/channel-recovery-policy'
import type {
  MonitoringFormInput,
  MonitoringFormValues,
} from './monitoring-settings-section'

export function ChannelRecoveryFields(props: {
  control: Control<MonitoringFormInput, unknown, MonitoringFormValues>
}) {
  const { t } = useTranslation()
  return (
    <section
      className='space-y-4 rounded-lg border p-4'
      aria-label={t('Recovery by disable reason')}
    >
      <FormField
        control={props.control}
        name='recovery.enabled'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between gap-4'>
            <div className='space-y-1'>
              <FormLabel>{t('Recovery by disable reason')}</FormLabel>
              <FormDescription>
                {t(
                  'Test auto-disabled channels independently of scheduled channel tests. Manual disables are never restored. When enabled, these rules replace the legacy re-enable switch.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </FormItem>
        )}
      />
      <p className='text-muted-foreground text-sm'>
        {t(
          'Recovery tests send real upstream requests. Checks run when due, with a 30-second timeout. Failed checks reset the success count. These rules do not add auto-disable triggers.'
        )}
      </p>
      {(
        Object.keys(recoveryReasonLabels) as Array<
          keyof RecoveryPolicy['rules']
        >
      ).map((reason) => (
        <div
          key={reason}
          className='grid items-start gap-4 rounded-md border p-3 md:grid-cols-3'
        >
          <FormField
            control={props.control}
            name={`recovery.rules.${reason}.enabled`}
            render={({ field }) => (
              <FormItem className='flex items-center justify-between gap-3 pt-1'>
                <FormLabel>{t(recoveryReasonLabels[reason])}</FormLabel>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />
          <FormField
            control={props.control}
            name={`recovery.rules.${reason}.interval_minutes`}
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Recovery interval (minutes)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={10080}
                    step={1}
                    {...field}
                    value={Number.isFinite(field.value) ? field.value : ''}
                    onChange={(event) =>
                      field.onChange(
                        event.target.value === ''
                          ? NaN
                          : Number(event.target.value)
                      )
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={props.control}
            name={`recovery.rules.${reason}.successes_required`}
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Consecutive successful checks')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={10}
                    step={1}
                    {...field}
                    value={Number.isFinite(field.value) ? field.value : ''}
                    onChange={(event) =>
                      field.onChange(
                        event.target.value === ''
                          ? NaN
                          : Number(event.target.value)
                      )
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      ))}
    </section>
  )
}
