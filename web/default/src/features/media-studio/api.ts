import { api } from '@/lib/api'
import type {
  VideoGroupCapability,
  VideoSubmitPayload,
  VideoTaskResponse,
} from './types'

const VIDEO_CAPABILITIES_ENDPOINT = '/api/user/playground/video-capabilities'
const VIDEOS_ENDPOINT = '/pg/videos'

export async function getVideoCapabilities(): Promise<VideoGroupCapability[]> {
  const response = await api.get(VIDEO_CAPABILITIES_ENDPOINT)
  if (!response.data?.success || !Array.isArray(response.data.data)) {
    return []
  }
  return response.data.data as VideoGroupCapability[]
}

export async function submitVideo(
  payload: VideoSubmitPayload | FormData
): Promise<VideoTaskResponse> {
  const response = await api.post(VIDEOS_ENDPOINT, payload, {
    skipErrorHandler: true,
  })
  return response.data as VideoTaskResponse
}

export async function getVideoTask(taskId: string): Promise<VideoTaskResponse> {
  const response = await api.get(`${VIDEOS_ENDPOINT}/${taskId}`, {
    disableDuplicate: true,
    skipErrorHandler: true,
  })
  return response.data as VideoTaskResponse
}
