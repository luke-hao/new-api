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
import { create } from 'zustand'
import { DEFAULT_IMAGE_CONFIG } from '../constants'
import {
  loadImageConfig,
  loadImageHistory,
  saveImageConfig,
  saveImageHistory,
} from '../lib'
import type { ImageHistoryItem, ImagePlaygroundConfig } from '../types'

type ImageHistoryUpdater =
  | ImageHistoryItem[]
  | ((previous: ImageHistoryItem[]) => ImageHistoryItem[])

interface ImagePlaygroundState {
  imageConfig: ImagePlaygroundConfig
  imageHistory: ImageHistoryItem[]
  updateImageConfig: <K extends keyof ImagePlaygroundConfig>(
    key: K,
    value: ImagePlaygroundConfig[K]
  ) => void
  updateImageConfigValues: (values: Partial<ImagePlaygroundConfig>) => void
  updateImageHistory: (updater: ImageHistoryUpdater) => void
  clearImageHistory: () => void
  resetImageConfig: () => void
}

function loadInitialHistory(): ImageHistoryItem[] {
  const stored = loadImageHistory() ?? []
  let changed = false
  const history = stored.map((item) => {
    if (item.status !== 'pending') return item
    changed = true
    return {
      ...item,
      status: 'failed' as const,
      updatedAt: Date.now(),
    }
  })
  if (changed) saveImageHistory(history)
  return history
}

export const useImagePlaygroundStore = create<ImagePlaygroundState>()(
  (set) => ({
    imageConfig: { ...DEFAULT_IMAGE_CONFIG, ...loadImageConfig() },
    imageHistory: loadInitialHistory(),
    updateImageConfig: (key, value) =>
      set((state) => {
        const imageConfig = { ...state.imageConfig, [key]: value }
        saveImageConfig(imageConfig)
        return { imageConfig }
      }),
    updateImageConfigValues: (values) =>
      set((state) => {
        const imageConfig = { ...state.imageConfig, ...values }
        saveImageConfig(imageConfig)
        return { imageConfig }
      }),
    updateImageHistory: (updater) =>
      set((state) => {
        const imageHistory =
          typeof updater === 'function' ? updater(state.imageHistory) : updater
        saveImageHistory(imageHistory)
        return { imageHistory }
      }),
    clearImageHistory: () =>
      set(() => {
        saveImageHistory([])
        return { imageHistory: [] }
      }),
    resetImageConfig: () =>
      set(() => {
        saveImageConfig(DEFAULT_IMAGE_CONFIG)
        return { imageConfig: DEFAULT_IMAGE_CONFIG }
      }),
  })
)
