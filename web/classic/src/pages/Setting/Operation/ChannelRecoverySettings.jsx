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
import React, { useEffect, useState } from 'react';
import { Button, InputNumber, Switch } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

const initial = {
  enabled: false,
  rules: {
    balance: { enabled: true, interval_minutes: 30, successes_required: 1 },
    rate_limit: { enabled: true, interval_minutes: 2, successes_required: 2 },
    timeout: { enabled: true, interval_minutes: 5, successes_required: 2 },
    authentication: {
      enabled: false,
      interval_minutes: 60,
      successes_required: 1,
    },
    other: { enabled: false, interval_minutes: 10, successes_required: 2 },
  },
};
const labels = {
  balance: 'Insufficient upstream balance',
  rate_limit: 'Rate limited',
  timeout: 'Timeout or slow response',
  authentication: 'Authentication or permission error',
  other: 'Other channel errors',
};
function parse(raw) {
  try {
    const parsed = JSON.parse(raw);
    if (Object.keys(initial.rules).every((key) => parsed.rules?.[key]))
      return parsed;
  } catch {}
  return structuredClone(initial);
}
export default function ChannelRecoverySettings(props) {
  const { t } = useTranslation();
  const [policy, setPolicy] = useState(() => parse(props.value));
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    setPolicy(parse(props.value));
  }, [props.value]);
  const updateRule = (reason, key, value) =>
    setPolicy((current) => ({
      ...current,
      rules: {
        ...current.rules,
        [reason]: { ...current.rules[reason], [key]: value },
      },
    }));
  const save = async () => {
    const invalid = Object.values(policy.rules).some(
      (rule) =>
        !Number.isInteger(rule.interval_minutes) ||
        rule.interval_minutes < 1 ||
        rule.interval_minutes > 10080 ||
        !Number.isInteger(rule.successes_required) ||
        rule.successes_required < 1 ||
        rule.successes_required > 10,
    );
    if (invalid)
      return showError(t('重试间隔须为 1–10080 分钟，连续成功次数须为 1–10'));
    setSaving(true);
    try {
      const { data } = await API.put('/api/option/', {
        key: 'ChannelRecoveryPolicy',
        value: JSON.stringify(policy),
      });
      if (!data.success) return showError(data.message);
      showSuccess(t('保存成功'));
      props.refresh();
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  };
  return (
    <section
      className='mt-6 space-y-4'
      aria-label={t('Recovery by disable reason')}
    >
      <div className='flex items-center justify-between gap-4'>
        <h3>{t('Recovery by disable reason')}</h3>
        <Switch
          aria-label={t('Recovery by disable reason')}
          checked={policy.enabled}
          onChange={(enabled) =>
            setPolicy((current) => ({ ...current, enabled }))
          }
        />
      </div>
      <p>
        {t(
          'Test auto-disabled channels independently of scheduled channel tests. Manual disables are never restored. When enabled, these rules replace the legacy re-enable switch.',
        )}
      </p>
      <p>
        {t(
          'Recovery tests send real upstream requests. Checks run when due, with a 30-second timeout. Failed checks reset the success count. These rules do not add auto-disable triggers.',
        )}
      </p>
      {Object.keys(labels).map((reason) => (
        <div
          key={reason}
          className='grid grid-cols-1 gap-4 rounded-lg border p-3 md:grid-cols-3'
        >
          <div className='flex items-center justify-between gap-3'>
            <span>{t(labels[reason])}</span>
            <Switch
              aria-label={t(labels[reason])}
              checked={policy.rules[reason].enabled}
              onChange={(value) => updateRule(reason, 'enabled', value)}
            />
          </div>
          <label>
            {t('Recovery interval (minutes)')}
            <InputNumber
              min={1}
              max={10080}
              precision={0}
              value={policy.rules[reason].interval_minutes}
              onChange={(value) =>
                updateRule(reason, 'interval_minutes', value)
              }
            />
          </label>
          <label>
            {t('Consecutive successful checks')}
            <InputNumber
              min={1}
              max={10}
              precision={0}
              value={policy.rules[reason].successes_required}
              onChange={(value) =>
                updateRule(reason, 'successes_required', value)
              }
            />
          </label>
        </div>
      ))}
      <Button loading={saving} onClick={save}>
        {t('保存恢复规则')}
      </Button>
    </section>
  );
}
