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
import { lazy, Suspense } from 'react'

interface LazyLobeIconProps {
  iconName: string
  size?: number
}

const LobeIconRenderer = lazy(() =>
  import('@/lib/lobe-icon').then(({ getLobeIcon }) => ({
    default: ({ iconName, size = 20 }: LazyLobeIconProps) =>
      getLobeIcon(iconName, size),
  }))
)

function IconFallback({ iconName, size = 20 }: LazyLobeIconProps) {
  const label = iconName.trim().charAt(0).toUpperCase() || '?'

  return (
    <span
      aria-hidden='true'
      className='text-muted-foreground inline-flex items-center justify-center text-xs font-semibold'
      style={{ width: size, height: size }}
    >
      {label}
    </span>
  )
}

export function LazyLobeIcon({ iconName, size = 20 }: LazyLobeIconProps) {
  return (
    <Suspense fallback={<IconFallback iconName={iconName} size={size} />}>
      <LobeIconRenderer iconName={iconName} size={size} />
    </Suspense>
  )
}
