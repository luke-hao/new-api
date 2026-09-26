/*
Copyright (C) 2025 QuantumNous

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
import React, { useState } from 'react';
import { Button, Input } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

export default function AutoGroupEditor({ groups, value, onChange }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const move = (index, offset) => {
    const next = [...value];
    [next[index], next[index + offset]] = [next[index + offset], next[index]];
    onChange(next);
  };
  return (
    <div className='space-y-3 py-3' data-testid='classic-auto-group-editor'>
      <strong>{t('自动分组优先级')}</strong>
      <p className='text-xs'>
        {t('从上到下选择文字或生图分组，按实际使用分组计费')}
      </p>
      {value.map((name, index) => {
        const option = groups.find((group) => group.value === name);
        return (
          <div
            key={name}
            className='flex items-center gap-2 rounded border p-2'
          >
            <div className='min-w-0 flex-1 break-words'>
              {index + 1}. {name}
              <div className='text-xs'>{option?.label}</div>
              <div className='text-xs'>
                {option?.auto_types
                  ?.map((type) => (type === 'image' ? t('图片') : t('文字')))
                  .join(' / ')}
              </div>
              <div className='text-xs'>
                {option?.auto_eligible
                  ? String(option.ratio) + 'x'
                  : t('分组已失效、权限已撤销或暂无文字及生图模型')}
              </div>
            </div>
            <Button
              type='tertiary'
              size='small'
              aria-label={t('上移') + ': ' + name}
              disabled={index === 0}
              onClick={() => move(index, -1)}
            >
              ↑
            </Button>
            <Button
              type='tertiary'
              size='small'
              aria-label={t('下移') + ': ' + name}
              disabled={index === value.length - 1}
              onClick={() => move(index, 1)}
            >
              ↓
            </Button>
            <Button
              type='tertiary'
              size='small'
              aria-label={t('移除') + ': ' + name}
              onClick={() => onChange(value.filter((group) => group !== name))}
            >
              ×
            </Button>
          </div>
        );
      })}
      <Input
        aria-label={t('搜索分组')}
        placeholder={t('搜索分组')}
        value={search}
        onChange={setSearch}
      />
      <div className='max-h-48 space-y-1 overflow-y-auto'>
        {groups
          .filter(
            (group) =>
              group.auto_eligible &&
              !value.includes(group.value) &&
              (group.value + ' ' + group.label)
                .toLowerCase()
                .includes(search.toLowerCase()),
          )
          .map((group) => (
            <Button
              key={group.value}
              block
              type='tertiary'
              onClick={() => onChange([...value, group.value])}
            >
              + {group.value} · {group.ratio}x
            </Button>
          ))}
      </div>
    </div>
  );
}
