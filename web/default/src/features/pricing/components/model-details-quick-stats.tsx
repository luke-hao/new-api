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
import {
  CalendarClock,
  FileText,
  Layers,
  Maximize2,
  Sparkles,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  formatTokenCount,
  formatYearMonth,
  type ModelMetadata,
} from '../lib/model-metadata'
import { ModalityIcons } from './model-details-modalities'

type QuickStatsProps = { metadata: ModelMetadata }
type Stat = {
  key: string
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: React.ReactNode
  title?: string
  hint?: string
}

function tokenValue(
  value: number,
  officialLabel: string,
  unknown: string
): string {
  if (officialLabel) return officialLabel
  if (value > 0) return formatTokenCount(value)
  return unknown
}

function exactTokenTitle(
  value: number,
  label: string,
  unknown: string
): string {
  if (value > 0) return `${new Intl.NumberFormat().format(value)} tokens`
  return label || unknown
}

function buildStats(
  metadata: ModelMetadata,
  t: (key: string) => string
): Stat[] {
  const unknown = t('Unknown')
  const hasModalities =
    metadata.input_modalities.length > 0 &&
    metadata.output_modalities.length > 0
  return [
    {
      key: 'context',
      icon: Layers,
      label: t('Context'),
      value: tokenValue(
        metadata.context_length,
        metadata.context_display,
        unknown
      ),
      title: exactTokenTitle(
        metadata.context_length,
        metadata.context_display,
        unknown
      ),
      hint: t('Context window'),
    },
    {
      key: 'max-output',
      icon: Maximize2,
      label: t('Max output'),
      value: tokenValue(
        metadata.max_output_tokens,
        metadata.max_output_display,
        unknown
      ),
      title: exactTokenTitle(
        metadata.max_output_tokens,
        metadata.max_output_display,
        unknown
      ),
      hint: t('Maximum tokens per response'),
    },
    {
      key: 'modalities',
      icon: FileText,
      label: t('Modalities'),
      value: hasModalities ? (
        <ModalityFlow
          input={metadata.input_modalities}
          output={metadata.output_modalities}
        />
      ) : (
        unknown
      ),
    },
    {
      key: 'knowledge',
      icon: Sparkles,
      label: t('Knowledge cutoff'),
      value: metadata.knowledge_cutoff
        ? formatYearMonth(metadata.knowledge_cutoff)
        : unknown,
      title: metadata.knowledge_cutoff || undefined,
    },
    {
      key: 'release',
      icon: CalendarClock,
      label: t('Released'),
      value: metadata.release_date
        ? formatYearMonth(metadata.release_date)
        : unknown,
      title: metadata.release_date || undefined,
    },
  ]
}

function ModalityFlow(props: {
  input: ModelMetadata['input_modalities']
  output: ModelMetadata['output_modalities']
}) {
  return (
    <span className='inline-flex items-center gap-1 align-middle'>
      <ModalityIcons modalities={props.input} className='size-3.5' />
      <span className='text-muted-foreground/40'>→</span>
      <ModalityIcons modalities={props.output} className='size-3.5' />
    </span>
  )
}

export function ModelDetailsQuickStats(props: QuickStatsProps) {
  const { t } = useTranslation()
  const stats = buildStats(props.metadata, t)
  return (
    <section className='space-y-2' data-testid='model-specifications'>
      <div className='bg-muted/20 grid grid-cols-2 gap-px overflow-hidden rounded-lg border @md/details:grid-cols-3 @2xl/details:grid-cols-5'>
        {stats.map((stat) => {
          const Icon = stat.icon
          return (
            <div
              key={stat.key}
              data-stat={stat.key}
              className='bg-background flex min-w-0 flex-col gap-0.5 px-3 py-2.5'
            >
              <span className='text-muted-foreground inline-flex min-w-0 items-center gap-1 text-[10px] font-medium tracking-wider uppercase'>
                <Icon className='size-3 shrink-0' />
                <span className='truncate'>{stat.label}</span>
              </span>
              <span
                className='text-foreground truncate text-sm font-semibold tabular-nums'
                title={stat.title}
              >
                {stat.value}
              </span>
              {stat.hint && (
                <span className='text-muted-foreground/60 truncate text-[10px]'>
                  {stat.hint}
                </span>
              )}
            </div>
          )
        })}
      </div>
      <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]'>
        <span>
          {t(
            'Unverified specifications are shown as Unknown; channel limits may differ.'
          )}
        </span>
        {props.metadata.sources.map((url, index) => (
          <a
            key={url}
            href={url}
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary underline underline-offset-2'
          >
            {t('Official documentation')} {index + 1}
          </a>
        ))}
        {props.metadata.checked_at && (
          <span>
            {t('Verified on')} {props.metadata.checked_at}
          </span>
        )}
      </div>
    </section>
  )
}
