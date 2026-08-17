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
import { useState, useCallback } from 'react'
import {
  DEFAULT_CONFIG,
  DEFAULT_IMAGE_CONFIG,
  DEFAULT_PARAMETER_ENABLED,
} from '../constants'
import {
  loadConfig,
  saveConfig,
  loadParameterEnabled,
  saveParameterEnabled,
  loadMessages,
  saveMessages,
  loadImageConfig,
  saveImageConfig,
  loadImageHistory,
  saveImageHistory,
} from '../lib'
import type {
  ImageHistoryItem,
  ImagePlaygroundConfig,
  Message,
  PlaygroundConfig,
  ParameterEnabled,
  ModelOption,
  GroupOption,
} from '../types'

/**
 * Main state management hook for playground
 */
export function usePlaygroundState() {
  // Load initial state from localStorage
  const [config, setConfig] = useState<PlaygroundConfig>(() => {
    const savedConfig = loadConfig()
    return { ...DEFAULT_CONFIG, ...savedConfig }
  })

  const [parameterEnabled, setParameterEnabled] = useState<ParameterEnabled>(
    () => {
      const saved = loadParameterEnabled()
      return { ...DEFAULT_PARAMETER_ENABLED, ...saved }
    }
  )

  const [messages, setMessages] = useState<Message[]>(() => {
    return loadMessages() || []
  })

  const [imageConfig, setImageConfig] = useState<ImagePlaygroundConfig>(() => {
    const savedConfig = loadImageConfig()
    return { ...DEFAULT_IMAGE_CONFIG, ...savedConfig }
  })

  const [imageHistory, setImageHistory] = useState<ImageHistoryItem[]>(() => {
    return loadImageHistory() || []
  })

  const [models, setModels] = useState<ModelOption[]>([])
  const [groups, setGroups] = useState<GroupOption[]>([])

  // Update config with automatic save
  const updateConfig = useCallback(
    <K extends keyof PlaygroundConfig>(key: K, value: PlaygroundConfig[K]) => {
      setConfig((prev) => {
        const updated = { ...prev, [key]: value }
        saveConfig(updated)
        return updated
      })
    },
    []
  )

  // Update parameter enabled with automatic save
  const updateParameterEnabled = useCallback(
    (key: keyof ParameterEnabled, value: boolean) => {
      setParameterEnabled((prev) => {
        const updated = { ...prev, [key]: value }
        saveParameterEnabled(updated)
        return updated
      })
    },
    []
  )

  // Update messages with automatic save
  const updateMessages = useCallback(
    (updater: Message[] | ((prev: Message[]) => Message[])) => {
      setMessages((prev) => {
        const newMessages =
          typeof updater === 'function' ? updater(prev) : updater
        saveMessages(newMessages)
        return newMessages
      })
    },
    []
  )

  const updateImageConfig = useCallback(
    <K extends keyof ImagePlaygroundConfig>(
      key: K,
      value: ImagePlaygroundConfig[K]
    ) => {
      setImageConfig((prev) => {
        const updated = { ...prev, [key]: value }
        saveImageConfig(updated)
        return updated
      })
    },
    []
  )

  const updateImageConfigValues = useCallback(
    (values: Partial<ImagePlaygroundConfig>) => {
      setImageConfig((prev) => {
        const updated = { ...prev, ...values }
        saveImageConfig(updated)
        return updated
      })
    },
    []
  )

  const updateImageHistory = useCallback(
    (
      updater:
        | ImageHistoryItem[]
        | ((prev: ImageHistoryItem[]) => ImageHistoryItem[])
    ) => {
      setImageHistory((prev) => {
        const newHistory =
          typeof updater === 'function' ? updater(prev) : updater
        saveImageHistory(newHistory)
        return newHistory
      })
    },
    []
  )

  // Clear all messages
  const clearMessages = useCallback(() => {
    updateMessages([])
  }, [updateMessages])

  const clearImageHistory = useCallback(() => {
    updateImageHistory([])
  }, [updateImageHistory])

  // Reset config to defaults
  const resetConfig = useCallback(() => {
    setConfig(DEFAULT_CONFIG)
    setParameterEnabled(DEFAULT_PARAMETER_ENABLED)
    saveConfig(DEFAULT_CONFIG)
    saveParameterEnabled(DEFAULT_PARAMETER_ENABLED)
  }, [])

  const resetImageConfig = useCallback(() => {
    setImageConfig(DEFAULT_IMAGE_CONFIG)
    saveImageConfig(DEFAULT_IMAGE_CONFIG)
  }, [])

  return {
    // State
    config,
    parameterEnabled,
    messages,
    imageConfig,
    imageHistory,
    models,
    groups,

    // Setters
    setModels,
    setGroups,

    // Actions
    updateConfig,
    updateParameterEnabled,
    updateMessages,
    updateImageConfig,
    updateImageConfigValues,
    updateImageHistory,
    clearMessages,
    clearImageHistory,
    resetConfig,
    resetImageConfig,
  }
}
