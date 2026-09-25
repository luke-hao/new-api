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
import { useQuery } from '@tanstack/react-query'
import { Copy, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserModels } from '@/lib/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog } from '@/components/dialog'
import { protocolUrl } from '../../lib/endpoints'
import { useApiKeys } from '../api-keys-provider'

export function KeySetupDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, selectedEndpoint, resolveRealKey } =
    useApiKeys()
  const [app, setApp] = useState<'codex' | 'claude' | 'curl'>('codex')
  const [model, setModel] = useState('')
  const [copying, setCopying] = useState(false)
  const { data } = useQuery({
    queryKey: ['user-models-setup'],
    queryFn: getUserModels,
    enabled: open === 'setup',
  })
  const models = (data?.data || []).filter(
    (name) =>
      !currentRow?.model_limits_enabled ||
      (currentRow.model_limits || '').split(',').includes(name)
  )
  const selectedModel = model.trim() || models[0] || 'MODEL'
  const generate = (key: string) => {
    if (app === 'claude')
      return JSON.stringify(
        {
          env: {
            ANTHROPIC_BASE_URL: selectedEndpoint,
            ANTHROPIC_AUTH_TOKEN: key,
            ANTHROPIC_MODEL: selectedModel,
          },
        },
        null,
        2
      )
    if (app === 'codex')
      return [
        '# ~/.codex/config.toml',
        'model = ' + JSON.stringify(selectedModel),
        'model_provider = "kele"',
        '',
        '[model_providers.kele]',
        'name = "Kele AI"',
        'base_url = ' + JSON.stringify(protocolUrl(selectedEndpoint, 'openai')),
        'wire_api = "responses"',
        'env_key = "KELE_API_KEY"',
        '',
        '# macOS / Linux',
        "export KELE_API_KEY='" + key + "'",
        '# PowerShell',
        "$env:KELE_API_KEY='" + key + "'",
      ].join('\n')
    return (
      'curl ' +
      JSON.stringify(protocolUrl(selectedEndpoint, 'openai') + '/responses') +
      ' \\\n  -H ' +
      JSON.stringify('Authorization: Bearer ' + key) +
      ' \\\n  -H "Content-Type: application/json" \\\n  -d \'' +
      JSON.stringify({ model: selectedModel, input: 'Hello' }).replaceAll(
        "'",
        "'\\''"
      ) +
      "'"
    )
  }
  const copy = async () => {
    if (!currentRow || copying) return
    setCopying(true)
    try {
      const key = await resolveRealKey(currentRow.id)
      if (!key) return
      const ok = await copyToClipboard(generate(key))
      if (ok) toast.success(t('Copied'))
      else toast.error(t('Failed to copy keys'))
    } finally {
      setCopying(false)
    }
  }
  return (
    <Dialog
      open={open === 'setup'}
      onOpenChange={(value) => !value && setOpen(null)}
      title={t('Quick setup')}
      description={t(
        'Use your selected endpoint with this API key. Running examples consumes quota.'
      )}
      footer={
        <Button disabled={copying} onClick={() => void copy()}>
          {copying ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <Copy className='size-4' />
          )}
          {t('Copy configuration with key')}
        </Button>
      }
    >
      <div className='space-y-4'>
        <div className='flex gap-1'>
          {(['codex', 'claude', 'curl'] as const).map((value) => (
            <Button
              key={value}
              variant={app === value ? 'secondary' : 'ghost'}
              aria-pressed={app === value}
              onClick={() => setApp(value)}
            >
              {value === 'codex'
                ? 'Codex'
                : value === 'claude'
                  ? 'Claude Code'
                  : 'cURL'}
            </Button>
          ))}
        </div>
        <div className='bg-muted/30 rounded-lg border p-3 font-mono text-xs break-all'>
          {selectedEndpoint}
        </div>
        <div className='space-y-2'>
          <Label htmlFor='setup-model'>{t('Model')}</Label>
          <Input
            id='setup-model'
            list='setup-models'
            value={model}
            placeholder={selectedModel}
            onChange={(event) => setModel(event.target.value)}
          />
          <datalist id='setup-models'>
            {models.map((name) => (
              <option key={name} value={name} />
            ))}
          </datalist>
        </div>
        <p className='text-muted-foreground text-xs'>
          {t(
            app === 'claude'
              ? 'Merge the env fields into ~/.claude/settings.json.'
              : app === 'codex'
                ? 'Merge the TOML section into config.toml, then set the environment variable in your terminal.'
                : 'Run this command in a macOS or Linux terminal. Choose a model supported by your group.'
          )}
        </p>
        <pre className='bg-muted/50 overflow-x-auto rounded-xl p-4 text-xs leading-relaxed'>
          <code>{generate('YOUR_API_KEY')}</code>
        </pre>
      </div>
    </Dialog>
  )
}
