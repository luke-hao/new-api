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
import type {
  ImagePlaygroundConfig,
  PlaygroundConfig,
  ParameterEnabled,
} from './types'

// Message constants
export const MESSAGE_ROLES = {
  USER: 'user',
  ASSISTANT: 'assistant',
  SYSTEM: 'system',
} as const

export const MESSAGE_STATUS = {
  LOADING: 'loading',
  STREAMING: 'streaming',
  COMPLETE: 'complete',
  ERROR: 'error',
} as const

// API endpoints
export const API_ENDPOINTS = {
  CHAT_COMPLETIONS: '/pg/chat/completions',
  IMAGE_GENERATIONS: '/pg/images/generations',
  IMAGE_EDITS: '/pg/images/edits',
  USER_MODELS: '/api/user/models',
  USER_GROUPS: '/api/user/self/groups',
  IMAGE_CAPABILITIES: '/api/user/playground/image-capabilities',
} as const

// Default group — uses 'default' as the safe fallback; auto-group is
// only selected when the backend confirms it is available for the user.
export const DEFAULT_GROUP = 'default' as const

// Default configuration
export const DEFAULT_CONFIG: PlaygroundConfig = {
  model: 'gpt-4o',
  group: DEFAULT_GROUP,
  temperature: 0.7,
  top_p: 1,
  max_tokens: 4096,
  frequency_penalty: 0,
  presence_penalty: 0,
  seed: null,
  stream: true,
}

export const DEFAULT_IMAGE_CONFIG: ImagePlaygroundConfig = {
  model: 'gpt-image-2',
  group: DEFAULT_GROUP,
  mode: 'generate',
  size: 'auto',
  quality: 'auto',
  n: 1,
  aspectRatio: '1:1',
  imageSize: '1K',
}

export const DEFAULT_PARAMETER_ENABLED: ParameterEnabled = {
  temperature: true,
  top_p: true,
  max_tokens: false,
  frequency_penalty: true,
  presence_penalty: true,
  seed: false,
}

// Storage keys
export const STORAGE_KEYS = {
  CONFIG: 'playground_config',
  MESSAGES: 'playground_messages',
  PARAMETER_ENABLED: 'playground_parameter_enabled',
  IMAGE_CONFIG: 'playground_image_config',
  IMAGE_HISTORY: 'playground_image_history',
} as const

export const IMAGE_MODEL_PRIORITY = [
  'gpt-image-2',
  'gpt-image-2-2026-04-21',
  'chatgpt-image-latest',
  'gpt-image-1',
  'gpt-image-1.5',
  'gpt-image-1-mini',
  'dall-e-3',
  'dall-e-2',
] as const

export const IMAGE_SIZE_OPTIONS = [
  'auto',
  '1024x1024',
  '1024x1536',
  '1536x1024',
] as const

export const GPT_IMAGE_2_SIZE_OPTIONS = [
  'auto',
  '1024x1024',
  '1024x1536',
  '1536x1024',
  '2048x2048',
  '2560x1440',
  '1440x2560',
  '3840x2160',
  '2160x3840',
] as const

export const GPT_IMAGE_2_CUSTOM_SIZE_DEFAULT = '3840x2160'

export const GPT_IMAGE_2_SIZE_LIMITS = {
  MIN_DIMENSION: 16,
  MAX_DIMENSION: 3840,
  MIN_PIXELS: 655360,
  MAX_PIXELS: 8294400,
  MAX_ASPECT_RATIO: 3,
} as const

export const IMAGE_QUALITY_OPTIONS = ['auto', 'low', 'medium', 'high'] as const

export const GEMINI_ASPECT_RATIO_OPTIONS = [
  '1:1',
  '2:3',
  '3:2',
  '3:4',
  '4:3',
  '4:5',
  '5:4',
  '9:16',
  '16:9',
  '21:9',
] as const

export const GEMINI_IMAGE_SIZE_OPTIONS = ['1K', '2K', '4K'] as const

// Error messages
export const ERROR_MESSAGES = {
  API_REQUEST_ERROR: 'Request error occurred',
  NETWORK_ERROR: 'Network connection failed or server not responding',
  PARSE_ERROR: 'Error parsing response data',
  STREAM_START_ERROR: 'Error establishing connection',
  CONNECTION_CLOSED: 'Connection closed',
  INTERRUPTED: 'Generation was interrupted',
} as const

// Message action button styles
export const MESSAGE_ACTION_BUTTON_STYLES = {
  BASE: 'size-7 text-muted-foreground hover:text-foreground',
  DELETE: 'size-7 text-muted-foreground hover:text-destructive',
  ICON: 'size-4',
} as const

// Message action labels
export const MESSAGE_ACTION_LABELS = {
  COPY: 'Copy',
  COPIED: 'Copied!',
  REGENERATE: 'Regenerate',
  EDIT: 'Edit',
  DELETE: 'Delete',
  NO_CONTENT: 'No content to copy',
  WAIT_GENERATION: 'Please wait for the current generation to complete',
} as const
