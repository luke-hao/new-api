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
// Message types
export type MessageRole = 'user' | 'assistant' | 'system'

export type MessageStatus = 'loading' | 'streaming' | 'complete' | 'error'

export interface MessageVersion {
  id: string
  content: string
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  sources?: { href: string; title: string }[]
  reasoning?: {
    content: string
    duration: number
  }
  isReasoningStreaming?: boolean
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
}

// API payload types
export interface ChatCompletionMessage {
  role: MessageRole
  content: string | ContentPart[]
}

export interface ContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: {
    url: string
  }
}

export interface ChatCompletionRequest {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  frequency_penalty?: number
  presence_penalty?: number
  seed?: number
  extra_body?: {
    google?: {
      image_config?: {
        aspect_ratio?: string
        image_size?: GeminiImageSize
      }
    }
  }
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    delta: {
      role?: MessageRole
      content?: string
      reasoning_content?: string
    }
    finish_reason: string | null
  }>
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: {
      role: MessageRole
      content: string
      reasoning_content?: string
    }
    finish_reason: string
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

// Image API types
export type ImageMode = 'generate' | 'edit'

export type ImageSize = string

export type ImageQuality = 'auto' | 'low' | 'medium' | 'high'

export type GeminiImageSize = '1K' | '2K' | '4K'

export type ImageProtocol = 'image_api' | 'gemini_chat'

export type ImageParameterProfile =
  | 'gpt_image_2'
  | 'gemini_image'
  | 'legacy_image'

export interface ImagePlaygroundConfig {
  model: string
  group: string
  mode: ImageMode
  size: ImageSize
  quality: ImageQuality
  n: number
  aspectRatio: string
  imageSize: GeminiImageSize
}

export interface ImageGenerationRequest {
  model: string
  group?: string
  prompt: string
  size?: ImageSize
  quality?: ImageQuality
  n?: number
}

export interface ImageData {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface ImageResponse {
  created?: number
  data: ImageData[]
}

export interface ImageResult {
  b64Json?: string
  url?: string
  dataUrl?: string
  revisedPrompt?: string
}

export type ImageTaskStatus = 'pending' | 'completed' | 'failed'

export interface ImageHistoryItem {
  id: string
  mode: ImageMode
  prompt: string
  model: string
  group: string
  protocol?: ImageProtocol
  profile?: ImageParameterProfile
  size?: ImageSize
  quality?: ImageQuality
  aspectRatio?: string
  imageSize?: GeminiImageSize
  n: number
  createdAt: number
  updatedAt?: number
  status?: ImageTaskStatus
  error?: string
  sourceImages?: Array<{
    name: string
    mediaType?: string
    dataUrl: string
  }>
  results: ImageResult[]
}

// Configuration types
export interface PlaygroundConfig {
  model: string
  group: string
  temperature: number
  top_p: number
  max_tokens: number
  frequency_penalty: number
  presence_penalty: number
  seed: number | null
  stream: boolean
}

export interface ParameterEnabled {
  temperature: boolean
  top_p: boolean
  max_tokens: boolean
  frequency_penalty: boolean
  presence_penalty: boolean
  seed: boolean
}

// Model and group options
export interface ModelOption {
  label: string
  value: string
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}

export interface ImageModelCapability {
  model: string
  protocol: ImageProtocol
  profile: ImageParameterProfile
  modes: ImageMode[]
  fixed_image_size?: '2K' | '4K'
}

export interface ImageGroupCapability {
  group: string
  desc: string
  ratio: number | string
  models: ImageModelCapability[]
}
