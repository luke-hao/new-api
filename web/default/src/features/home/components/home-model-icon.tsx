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
import type { ComponentType } from 'react'
import ClaudeColor from '@lobehub/icons/es/Claude/components/Color'
import GeminiColor from '@lobehub/icons/es/Gemini/components/Color'
import KlingColor from '@lobehub/icons/es/Kling/components/Color'
import OpenAIMono from '@lobehub/icons/es/OpenAI/components/Mono'
import XAIMono from '@lobehub/icons/es/XAI/components/Mono'

type BrandIcon = ComponentType<{ size?: number }>

const HOME_MODEL_ICONS: Record<string, BrandIcon> = {
  OpenAI: OpenAIMono,
  'Claude.Color': ClaudeColor,
  'Gemini.Color': GeminiColor,
  XAI: XAIMono,
  'Kling.Color': KlingColor,
}

type HomeModelIconProps = {
  iconName: string
  size?: number
}

export function HomeModelIcon({ iconName, size = 20 }: HomeModelIconProps) {
  const Icon = HOME_MODEL_ICONS[iconName]
  if (Icon) return <Icon size={size} />

  return (
    <span
      aria-hidden='true'
      className='text-muted-foreground inline-flex items-center justify-center text-xs font-semibold'
      style={{ width: size, height: size }}
    >
      {iconName.trim().charAt(0).toUpperCase() || '?'}
    </span>
  )
}
