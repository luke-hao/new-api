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
import { useTranslation } from 'react-i18next'

interface StatsProps {
  className?: string
}

export function Stats(_props: StatsProps) {
  const { t } = useTranslation()
  const stats = [
    { value: '0.06x', label: t('home.stats.codex') },
    { value: '< 5s', label: t('home.stats.firstToken') },
    { value: '0.09', label: t('home.stats.image') },
    { value: t('home.stats.invoiceValue'), label: t('home.stats.invoice') },
  ]

  return (
    <section className='border-border/50 bg-muted/20 relative z-10 border-b'>
      <div className='mx-auto grid max-w-6xl grid-cols-2 px-4 sm:px-6 md:grid-cols-4'>
        {stats.map((stat, index) => (
          <div
            key={stat.label}
            className={`flex min-h-28 flex-col justify-center py-5 text-center md:min-h-32 ${
              index % 2 === 1 ? 'border-border/50 border-l' : ''
            } ${index > 1 ? 'border-border/50 border-t md:border-t-0' : ''} ${
              index > 0 ? 'md:border-border/50 md:border-l' : ''
            }`}
          >
            <strong className='text-foreground font-mono text-2xl font-bold md:text-3xl'>
              {stat.value}
            </strong>
            <span className='text-muted-foreground mt-1.5 px-2 text-xs'>
              {stat.label}
            </span>
          </div>
        ))}
      </div>
    </section>
  )
}
