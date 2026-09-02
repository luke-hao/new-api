export type VideoMode =
  | 'text'
  | 'first_frame'
  | 'first_last'
  | 'reference'
  | 'video_edit'

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
  max_image_references: number
  max_video_references: number
  max_audio_references: number
  max_image_bytes: number
  max_video_bytes: number
  max_audio_bytes: number
  max_video_edit_bytes: number
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
  video_url?: string
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
  mode: VideoMode
  duration?: number
  seconds?: number | string
  size?: string
  metadata?: Record<string, unknown>
  extra?: Record<string, unknown>
}
