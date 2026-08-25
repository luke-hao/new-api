export type VideoMode = 'text' | 'image' | 'first_last'

export type VideoTaskStatus =
  | 'queued'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'timeout'

export interface VideoParametersCapability {
  durations?: number[]
  aspect_ratios?: string[]
  resolutions?: string[]
  supports_seed: boolean
  max_input_references: number
}

export interface VideoModelCapability {
  model: string
  profile: string
  modes: VideoMode[]
  parameters: VideoParametersCapability
}

export interface VideoGroupCapability {
  group: string
  desc: string
  ratio: number | string
  models: VideoModelCapability[]
}

export interface VideoTaskResponse {
  id?: string
  task_id?: string
  model?: string
  status?: string
  progress?: number
  metadata?: { url?: string }
  url?: string
  error?: { message?: string; code?: string } | null
}

export interface VideoStudioConfig {
  group: string
  model: string
  mode: VideoMode
  duration: number
  aspectRatio: string
  resolution: string
  seed: string
}

export interface VideoHistoryItem {
  id: string
  taskId: string
  prompt: string
  group: string
  model: string
  mode: VideoMode
  duration?: number
  aspectRatio?: string
  resolution?: string
  seed?: number
  sourceNames?: string[]
  status: VideoTaskStatus
  progress: number
  resultUrl?: string
  error?: string
  createdAt: number
  updatedAt: number
}

export interface VideoSubmitPayload {
  model: string
  group: string
  prompt: string
  duration?: number
  seconds?: string
  size?: string
  metadata?: Record<string, unknown>
}
