import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  CircleAlertIcon,
  ClapperboardIcon,
  DownloadIcon,
  ExternalLinkIcon,
  FileAudioIcon,
  FileVideoIcon,
  ImagePlusIcon,
  Loader2Icon,
  PlusIcon,
  RefreshCwIcon,
  SendIcon,
  Trash2Icon,
  XIcon,
} from 'lucide-react'
import { nanoid } from 'nanoid'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Textarea } from '@/components/ui/textarea'
import { getVideoTask, submitVideo } from './api'
import type {
  VideoGroupCapability,
  VideoHistoryItem,
  VideoMode,
  VideoModelCapability,
  VideoStudioConfig,
  VideoSubmitPayload,
  VideoTaskResponse,
  VideoTaskStatus,
} from './types'
import {
  type AvailableVideoCatalogSection,
  buildAvailableVideoCatalog,
  findAvailableVideoCatalogItem,
  findAvailableVideoCatalogItemByID,
  flattenAvailableVideoCatalog,
  VIDEO_GROUP_NAME,
} from './video-catalog'

const CONFIG_STORAGE_KEY = 'media_studio_video_config'
const HISTORY_STORAGE_KEY = 'media_studio_video_history'
const POLL_INTERVAL_MS = 5000
const TASK_TIMEOUT_MS = 2 * 60 * 60 * 1000
const MAX_HISTORY_ITEMS = 30
const DEFAULT_IMAGE_BYTES = 15 * 1024 * 1024
const DEFAULT_VIDEO_BYTES = 160 * 1024 * 1024
const DEFAULT_AUDIO_BYTES = 50 * 1024 * 1024

const DEFAULT_CONFIG: VideoStudioConfig = {
  group: VIDEO_GROUP_NAME,
  model: '',
  mode: 'text',
  duration: 5,
  aspectRatio: '16:9',
  resolution: '720p',
  seed: '',
}

type MediaKind = 'first' | 'last' | 'images' | 'videos' | 'audios'

type SelectedMedia = {
  id: string
  file: File
  previewUrl: string
}

type MediaState = Record<MediaKind, SelectedMedia[]>

type SelectOption = {
  label: string
  value: string
}

type SelectOptionGroup = {
  label: string
  options: SelectOption[]
}

function emptyMediaState(): MediaState {
  return { first: [], last: [], images: [], videos: [], audios: [] }
}

function catalogSelectGroups(
  sections: AvailableVideoCatalogSection[]
): SelectOptionGroup[] {
  return sections.map((section) => ({
    label: section.label,
    options: section.items.map((item) => ({
      label: item.label,
      value: item.id,
    })),
  }))
}

function normalizeMode(value: unknown): VideoMode {
  if (value === 'image') return 'first_frame'
  if (
    value === 'text' ||
    value === 'first_frame' ||
    value === 'first_last' ||
    value === 'reference' ||
    value === 'video_edit'
  ) {
    return value
  }
  return 'text'
}

function loadStoredConfig(): VideoStudioConfig {
  if (typeof window === 'undefined') return DEFAULT_CONFIG
  try {
    const raw = window.localStorage.getItem(CONFIG_STORAGE_KEY)
    if (!raw) return DEFAULT_CONFIG
    const parsed = JSON.parse(raw) as Partial<VideoStudioConfig> & {
      mode?: string
    }
    return {
      ...DEFAULT_CONFIG,
      ...parsed,
      group: VIDEO_GROUP_NAME,
      mode: normalizeMode(parsed.mode),
    }
  } catch {
    return DEFAULT_CONFIG
  }
}

function loadStoredHistory(): VideoHistoryItem[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(HISTORY_STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed.slice(0, MAX_HISTORY_ITEMS) : []
  } catch {
    return []
  }
}

function saveStoredValue(key: string, value: unknown) {
  try {
    window.localStorage.setItem(key, JSON.stringify(value))
  } catch {
    /* Browser storage may be unavailable or full. */
  }
}

function getErrorMessage(error: unknown, fallback: string) {
  const err = error as {
    response?: {
      data?: {
        error?: { message?: string }
        message?: string
      }
    }
    message?: string
  }
  return (
    err.response?.data?.error?.message ||
    err.response?.data?.message ||
    err.message ||
    fallback
  )
}

function normalizeStatus(status?: string): VideoTaskStatus {
  switch (status?.toLowerCase()) {
    case 'completed':
    case 'succeeded':
    case 'success':
      return 'completed'
    case 'failed':
    case 'failure':
    case 'cancelled':
    case 'canceled':
      return 'failed'
    case 'in_progress':
    case 'processing':
    case 'running':
      return 'in_progress'
    default:
      return 'queued'
  }
}

function normalizeProgress(value: unknown, fallback = 0) {
  const progress =
    typeof value === 'string' ? Number.parseFloat(value) : Number(value)
  if (!Number.isFinite(progress)) return fallback
  return Math.max(0, Math.min(100, progress))
}

function taskResponseUrl(response: VideoTaskResponse) {
  return (
    response.video_url || response.url || response.metadata?.url || undefined
  )
}

function ratioToPixelSize(aspectRatio: string) {
  const sizes: Record<string, string> = {
    '16:9': '1280x720',
    '9:16': '720x1280',
    '1:1': '1024x1024',
    '4:3': '1024x768',
    '3:4': '768x1024',
    '3:2': '1152x768',
    '2:3': '768x1152',
    '21:9': '1792x768',
    '9:21': '768x1792',
  }
  return sizes[aspectRatio] || '1280x720'
}

function requestSize(model: VideoModelCapability, config: VideoStudioConfig) {
  if (model.profile === 'ali' && config.mode === 'text') {
    return ratioToPixelSize(config.aspectRatio).replace('x', '*')
  }
  if (model.profile === 'sora' || model.profile === 'openai') {
    if (config.resolution === '1080p') {
      return config.aspectRatio === '9:16' ? '1024x1792' : '1792x1024'
    }
    return config.aspectRatio === '9:16' ? '720x1280' : '1280x720'
  }
  return model.parameters.resolutions?.length
    ? config.resolution
    : ratioToPixelSize(config.aspectRatio)
}

function resolveConfigForModel(
  config: VideoStudioConfig,
  group: string,
  model: VideoModelCapability
): VideoStudioConfig {
  const durations = model.parameters.durations ?? []
  const aspectRatios = model.parameters.aspect_ratios ?? []
  const resolutions = model.parameters.resolutions ?? []
  const currentMode = normalizeMode(config.mode)
  return {
    ...config,
    group,
    model: model.model,
    mode: model.modes.includes(currentMode)
      ? currentMode
      : (model.modes[0] ?? 'text'),
    duration: durations.includes(config.duration)
      ? config.duration
      : (durations[0] ?? config.duration),
    aspectRatio: aspectRatios.includes(config.aspectRatio)
      ? config.aspectRatio
      : (aspectRatios[0] ?? config.aspectRatio),
    resolution: resolutions.includes(config.resolution)
      ? config.resolution
      : (resolutions[0] ?? config.resolution),
  }
}

function normalizeTaskUpdate(
  item: VideoHistoryItem,
  response: VideoTaskResponse,
  now = Date.now()
): VideoHistoryItem {
  const status = normalizeStatus(response.status)
  const progress = normalizeProgress(
    response.progress,
    status === 'completed' ? 100 : 0
  )
  return {
    ...item,
    status,
    progress,
    resultUrl: taskResponseUrl(response) || item.resultUrl,
    error: response.error?.message || (status === 'failed' ? item.error : ''),
    updatedAt: now,
  }
}

function isActiveTask(item: VideoHistoryItem) {
  return item.status === 'queued' || item.status === 'in_progress'
}

function maxReferences(model: VideoModelCapability, kind: MediaKind) {
  const parameters = model.parameters
  switch (kind) {
    case 'first':
    case 'last':
      return 1
    case 'images':
      return (
        parameters.max_image_references || parameters.max_input_references || 0
      )
    case 'videos':
      return parameters.max_video_references || 0
    case 'audios':
      return parameters.max_audio_references || 0
  }
}

function maxBytes(
  model: VideoModelCapability,
  kind: MediaKind,
  mode: VideoMode
) {
  const parameters = model.parameters
  if (kind === 'first' || kind === 'last' || kind === 'images') {
    return parameters.max_image_bytes || DEFAULT_IMAGE_BYTES
  }
  if (kind === 'videos') {
    if (mode === 'video_edit') {
      return parameters.max_video_edit_bytes || 8 * 1024 * 1024
    }
    return parameters.max_video_bytes || DEFAULT_VIDEO_BYTES
  }
  return parameters.max_audio_bytes || DEFAULT_AUDIO_BYTES
}

function allowedKinds(mode: VideoMode): MediaKind[] {
  switch (mode) {
    case 'first_frame':
      return ['first']
    case 'first_last':
      return ['first', 'last']
    case 'reference':
      return ['images', 'videos', 'audios']
    case 'video_edit':
      return ['videos']
    default:
      return []
  }
}

function revokeMedia(items: SelectedMedia[]) {
  for (const item of items) URL.revokeObjectURL(item.previewUrl)
}

function trimMediaForMode(
  previous: MediaState,
  mode: VideoMode,
  model: VideoModelCapability
) {
  const allowed = new Set(allowedKinds(mode))
  const next = emptyMediaState()
  for (const kind of Object.keys(previous) as MediaKind[]) {
    if (!allowed.has(kind)) {
      revokeMedia(previous[kind])
      continue
    }
    const limit = maxReferences(model, kind)
    next[kind] = previous[kind].slice(0, limit)
    revokeMedia(previous[kind].slice(limit))
  }
  return next
}

function formatBytes(bytes: number) {
  const megabytes = Math.round((bytes / 1024 / 1024) * 10) / 10
  return String(megabytes) + ' MB'
}

async function prepareReferenceImage(
  file: File,
  aspectRatio: string
): Promise<Blob> {
  const match = aspectRatio.match(/^(\d+(?:\.\d+)?):(\d+(?:\.\d+)?)$/)
  if (!match) return file
  const wanted = Number(match[1]) / Number(match[2])
  if (!Number.isFinite(wanted) || wanted <= 0) return file

  const objectUrl = URL.createObjectURL(file)
  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const element = new Image()
      element.onload = () => resolve(element)
      element.onerror = () =>
        reject(new Error('Reference image could not be read'))
      element.src = objectUrl
    })
    const canvasWidth =
      wanted >= 1 ? 1024 : Math.max(1, Math.round(1024 * wanted))
    const canvasHeight =
      wanted >= 1 ? Math.max(1, Math.round(1024 / wanted)) : 1024
    const scale = Math.min(
      canvasWidth / image.naturalWidth,
      canvasHeight / image.naturalHeight
    )
    const width = Math.max(1, Math.round(image.naturalWidth * scale))
    const height = Math.max(1, Math.round(image.naturalHeight * scale))
    const canvas = document.createElement('canvas')
    canvas.width = canvasWidth
    canvas.height = canvasHeight
    const context = canvas.getContext('2d')
    if (!context) throw new Error('Reference image could not be processed')
    context.fillStyle = '#000'
    context.fillRect(0, 0, canvasWidth, canvasHeight)
    context.drawImage(
      image,
      Math.round((canvasWidth - width) / 2),
      Math.round((canvasHeight - height) / 2),
      width,
      height
    )
    return await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob(
        (blob) =>
          blob
            ? resolve(blob)
            : reject(new Error('Reference image could not be processed')),
        'image/jpeg',
        0.86
      )
    })
  } finally {
    URL.revokeObjectURL(objectUrl)
  }
}

export function VideoStudio({
  capabilities,
  isLoading = false,
}: {
  capabilities: VideoGroupCapability[]
  isLoading?: boolean
}) {
  const { t } = useTranslation()
  const [config, setConfig] = useState<VideoStudioConfig>(loadStoredConfig)
  const [history, setHistory] = useState<VideoHistoryItem[]>(loadStoredHistory)
  const [prompt, setPrompt] = useState('')
  const [media, setMedia] = useState<MediaState>(emptyMediaState)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pickerKindRef = useRef<MediaKind>('images')
  const mediaRef = useRef(media)
  const historyRef = useRef(history)

  const selectedGroup = useMemo(
    () => capabilities.find((item) => item.group === VIDEO_GROUP_NAME),
    [capabilities]
  )
  const availableCatalog = useMemo(
    () => buildAvailableVideoCatalog(selectedGroup?.models ?? []),
    [selectedGroup]
  )
  const availableModels = useMemo(
    () => flattenAvailableVideoCatalog(availableCatalog),
    [availableCatalog]
  )
  const selectedModel = useMemo(
    () =>
      availableModels.find((item) => item.model === config.model) ??
      availableModels[0],
    [availableModels, config.model]
  )
  const selectedCatalogItem = useMemo(
    () =>
      findAvailableVideoCatalogItem(
        availableCatalog,
        selectedModel?.model ?? ''
      ),
    [availableCatalog, selectedModel]
  )
  const effectiveConfig = useMemo(
    () =>
      selectedGroup && selectedModel
        ? resolveConfigForModel(config, selectedGroup.group, selectedModel)
        : config,
    [config, selectedGroup, selectedModel]
  )

  const updateConfig = useCallback((values: Partial<VideoStudioConfig>) => {
    setConfig((previous) => {
      const next = { ...previous, ...values }
      saveStoredValue(CONFIG_STORAGE_KEY, next)
      return next
    })
  }, [])

  const updateHistory = useCallback(
    (
      updater:
        | VideoHistoryItem[]
        | ((items: VideoHistoryItem[]) => VideoHistoryItem[])
    ) => {
      setHistory((previous) => {
        const next = typeof updater === 'function' ? updater(previous) : updater
        const limited = next.slice(0, MAX_HISTORY_ITEMS)
        historyRef.current = limited
        saveStoredValue(HISTORY_STORAGE_KEY, limited)
        return limited
      })
    },
    []
  )

  useEffect(() => {
    historyRef.current = history
  }, [history])

  useEffect(() => {
    mediaRef.current = media
  }, [media])

  useEffect(() => {
    return () => {
      for (const items of Object.values(mediaRef.current)) revokeMedia(items)
    }
  }, [])

  const pollActiveTasks = useCallback(async () => {
    const now = Date.now()
    const active = historyRef.current.filter(isActiveTask)
    if (active.length === 0) return
    const timedOutIds = new Set(
      active
        .filter((item) => now - item.createdAt >= TASK_TIMEOUT_MS)
        .map((item) => item.id)
    )
    if (timedOutIds.size > 0) {
      updateHistory((items) =>
        items.map((item) =>
          timedOutIds.has(item.id)
            ? {
                ...item,
                status: 'timeout',
                error: t('Task polling timed out'),
                updatedAt: now,
              }
            : item
        )
      )
    }
    const pollable = active.filter((item) => !timedOutIds.has(item.id))
    const updates = await Promise.all(
      pollable.map(async (item) => {
        try {
          return { id: item.id, response: await getVideoTask(item.taskId) }
        } catch {
          return null
        }
      })
    )
    const byID = new Map(
      updates
        .filter((item): item is NonNullable<typeof item> => item !== null)
        .map((item) => [item.id, item.response])
    )
    if (byID.size > 0) {
      updateHistory((items) =>
        items.map((item) => {
          const response = byID.get(item.id)
          return response ? normalizeTaskUpdate(item, response, now) : item
        })
      )
    }
  }, [t, updateHistory])

  useEffect(() => {
    void pollActiveTasks()
    const timer = window.setInterval(pollActiveTasks, POLL_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [pollActiveTasks])

  const applyModelConfig = useCallback(
    (model: VideoModelCapability) => {
      const next = resolveConfigForModel(
        effectiveConfig,
        VIDEO_GROUP_NAME,
        model
      )
      updateConfig(next)
      setMedia((previous) => trimMediaForMode(previous, next.mode, model))
    },
    [effectiveConfig, updateConfig]
  )

  const handleCatalogChange = (catalogID: string) => {
    const catalogItem = findAvailableVideoCatalogItemByID(
      availableCatalog,
      catalogID
    )
    const model = catalogItem?.models[0]
    if (model) applyModelConfig(model)
  }

  const handleModelChange = (modelName: string) => {
    const model = availableModels.find((item) => item.model === modelName)
    if (model) applyModelConfig(model)
  }

  const handleModeChange = (mode: VideoMode) => {
    if (!selectedModel) return
    updateConfig({ mode })
    setMedia((previous) => trimMediaForMode(previous, mode, selectedModel))
  }

  const openPicker = (kind: MediaKind) => {
    const input = fileInputRef.current
    if (!input) return
    pickerKindRef.current = kind
    input.accept =
      kind === 'videos' ? 'video/*' : kind === 'audios' ? 'audio/*' : 'image/*'
    input.multiple = kind === 'images' || kind === 'videos' || kind === 'audios'
    input.value = ''
    input.click()
  }

  const handleFiles = (files: FileList | null) => {
    if (!files || !selectedModel) return
    const kind = pickerKindRef.current
    const expectedPrefix =
      kind === 'videos' ? 'video/' : kind === 'audios' ? 'audio/' : 'image/'
    const byteLimit = maxBytes(selectedModel, kind, effectiveConfig.mode)
    const accepted = Array.from(files).filter((file) => {
      if (!file.type.toLowerCase().startsWith(expectedPrefix)) {
        toast.error(t('Unsupported media type') + ': ' + file.name)
        return false
      }
      if (file.size > byteLimit) {
        toast.error(
          file.name + ' ' + t('exceeds') + ' ' + formatBytes(byteLimit)
        )
        return false
      }
      return true
    })
    if (accepted.length === 0) return

    setMedia((previous) => {
      const next = { ...previous }
      const limit = maxReferences(selectedModel, kind)
      const additions = accepted.map((file) => ({
        id: nanoid(),
        file,
        previewUrl: URL.createObjectURL(file),
      }))
      if (kind === 'first' || kind === 'last') {
        revokeMedia(previous[kind])
        next[kind] = additions.slice(0, 1)
        revokeMedia(additions.slice(1))
        return next
      }
      const remaining = Math.max(0, limit - previous[kind].length)
      next[kind] = [...previous[kind], ...additions.slice(0, remaining)]
      revokeMedia(additions.slice(remaining))
      if (additions.length > remaining) {
        toast.error(t('Media limit reached') + ': ' + String(limit))
      }
      return next
    })
  }

  const removeMedia = (kind: MediaKind, id: string) => {
    setMedia((previous) => {
      const item = previous[kind].find((entry) => entry.id === id)
      if (item) URL.revokeObjectURL(item.previewUrl)
      return {
        ...previous,
        [kind]: previous[kind].filter((entry) => entry.id !== id),
      }
    })
  }

  const materialCount = Object.values(media).reduce(
    (total, items) => total + items.length,
    0
  )

  const validateSubmission = () => {
    switch (effectiveConfig.mode) {
      case 'first_frame':
        return media.first.length === 1
      case 'first_last':
        return media.first.length === 1 && media.last.length === 1
      case 'reference':
        return (
          media.images.length + media.videos.length + media.audios.length > 0
        )
      case 'video_edit':
        return media.videos.length > 0
      default:
        return materialCount === 0
    }
  }

  const submit = async () => {
    if (!selectedGroup || !selectedModel || !prompt.trim()) return
    if (!validateSubmission()) {
      toast.error(t('Add the required reference media'))
      return
    }
    const seedValue = effectiveConfig.seed.trim()
      ? Number(effectiveConfig.seed)
      : undefined
    if (
      seedValue !== undefined &&
      (!Number.isInteger(seedValue) || seedValue < 0)
    ) {
      toast.error(t('Seed must be a non-negative integer'))
      return
    }
    const size = requestSize(selectedModel, effectiveConfig)
    const extra: Record<string, unknown> = {}
    if (selectedModel.parameters.aspect_ratios?.length) {
      extra.aspect_ratio = effectiveConfig.aspectRatio
    }
    if (selectedModel.parameters.resolutions?.length) {
      extra.resolution = effectiveConfig.resolution
    }
    if (seedValue !== undefined) extra.seed = seedValue

    setIsSubmitting(true)
    try {
      let response: VideoTaskResponse
      if (materialCount === 0) {
        const payload: VideoSubmitPayload = {
          model: selectedModel.model,
          group: selectedGroup.group,
          prompt: prompt.trim(),
          mode: effectiveConfig.mode,
          duration: effectiveConfig.duration,
          seconds: effectiveConfig.duration,
          size,
          extra,
        }
        response = await submitVideo(payload)
      } else {
        const form = new FormData()
        form.append('model', selectedModel.model)
        form.append('group', selectedGroup.group)
        form.append('prompt', prompt.trim())
        form.append('mode', effectiveConfig.mode)
        form.append('duration', String(effectiveConfig.duration))
        form.append('seconds', String(effectiveConfig.duration))
        form.append('size', size)
        form.append('extra', JSON.stringify(extra))
        const appendImage = async (
          item: SelectedMedia,
          field: 'input_reference' | 'reference_images'
        ) => {
          const blob = await prepareReferenceImage(
            item.file,
            effectiveConfig.aspectRatio
          )
          const basename = item.file.name.replace(/\.[^.]+$/, '') || 'image'
          form.append(field, blob, basename + '.jpg')
        }
        if (effectiveConfig.mode === 'first_frame') {
          await appendImage(media.first[0], 'input_reference')
        } else if (effectiveConfig.mode === 'first_last') {
          await appendImage(media.first[0], 'input_reference')
          await appendImage(media.last[0], 'input_reference')
        } else {
          for (const item of media.images) {
            await appendImage(item, 'reference_images')
          }
          for (const item of media.videos) {
            form.append('reference_videos', item.file, item.file.name)
          }
          for (const item of media.audios) {
            form.append('reference_audios', item.file, item.file.name)
          }
        }
        response = await submitVideo(form)
      }
      const taskId = response.id || response.task_id
      if (!taskId) throw new Error(t('Video task ID was not returned'))

      const now = Date.now()
      const sourceNames = (
        Object.entries(media) as [MediaKind, SelectedMedia[]][]
      ).flatMap(([kind, items]) =>
        items.map((item) => kind + ': ' + item.file.name)
      )
      const item: VideoHistoryItem = {
        id: nanoid(),
        taskId,
        prompt: prompt.trim(),
        group: selectedGroup.group,
        model: selectedModel.model,
        mode: effectiveConfig.mode,
        duration: effectiveConfig.duration,
        aspectRatio: effectiveConfig.aspectRatio,
        resolution: effectiveConfig.resolution,
        seed: seedValue,
        sourceNames,
        status: response.error ? 'failed' : normalizeStatus(response.status),
        progress: Number(response.progress ?? 0),
        resultUrl: taskResponseUrl(response),
        error: response.error?.message,
        createdAt: now,
        updatedAt: now,
      }
      updateHistory((items) => [item, ...items])
      setPrompt('')
      toast.success(t('Video task submitted'))
    } catch (error) {
      toast.error(getErrorMessage(error, t('Video request failed')))
    } finally {
      setIsSubmitting(false)
    }
  }

  const prepareRetry = (item: VideoHistoryItem) => {
    updateConfig({
      group: item.group,
      model: item.model,
      mode: normalizeMode(item.mode),
      duration: item.duration ?? config.duration,
      aspectRatio: item.aspectRatio ?? config.aspectRatio,
      resolution: item.resolution ?? config.resolution,
      seed: item.seed === undefined ? '' : String(item.seed),
    })
    setPrompt(item.prompt)
    setMedia((previous) => {
      for (const items of Object.values(previous)) revokeMedia(items)
      return emptyMediaState()
    })
    toast.info(
      item.mode === 'text'
        ? t('Ready to retry')
        : t('Reattach source media to retry')
    )
  }

  const hasModels = availableModels.length > 0
  const submitDisabled =
    isSubmitting ||
    isLoading ||
    !hasModels ||
    !selectedModel ||
    !prompt.trim() ||
    !validateSubmission()

  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-y-auto'>
      <div className='flex-none'>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-5 px-4 py-4'>
          <div className='flex items-center justify-between gap-3 border-b pb-2'>
            <div className='min-w-0'>
              <h2 className='text-sm font-semibold'>
                {t('Generation history')}
              </h2>
              <p className='text-muted-foreground truncate text-xs'>
                {selectedModel?.model ?? t('No model')}
              </p>
            </div>
            <Button
              aria-label={t('Clear history')}
              disabled={history.length === 0 || isSubmitting}
              onClick={() => updateHistory([])}
              size='icon'
              title={t('Clear history')}
              type='button'
              variant='outline'
            >
              <Trash2Icon />
            </Button>
          </div>

          {!hasModels && !isLoading ? (
            <EmptyCanvas
              icon={<CircleAlertIcon />}
              title={t('No video models available')}
            />
          ) : history.length === 0 ? (
            <EmptyCanvas
              icon={<ClapperboardIcon />}
              title={t('Videos will appear here')}
            />
          ) : (
            <div className='grid gap-4 lg:grid-cols-2'>
              {history.map((item) => (
                <VideoHistoryCard
                  item={item}
                  key={item.id}
                  onRetry={() => prepareRetry(item)}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      <div className='border-border/70 bg-background/95 border-t px-3 py-3 backdrop-blur md:px-4 md:pb-4'>
        <div className='border-border mx-auto w-full max-w-7xl overflow-hidden rounded-lg border'>
          <Textarea
            autoCapitalize='off'
            autoComplete='off'
            autoCorrect='off'
            className='max-h-40 min-h-20 resize-y rounded-none border-0 px-4 py-3 shadow-none focus-visible:ring-0 md:text-base'
            disabled={isSubmitting || !hasModels}
            onChange={(event) => setPrompt(event.target.value)}
            onKeyDown={(event) => {
              if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                event.preventDefault()
                if (!submitDisabled) void submit()
              }
            }}
            placeholder={t('Describe the video to generate')}
            spellCheck={false}
            value={prompt}
          />
          {selectedModel && allowedKinds(effectiveConfig.mode).length > 0 && (
            <MaterialShelf
              media={media}
              mode={effectiveConfig.mode}
              model={selectedModel}
              note={selectedCatalogItem?.note ?? ''}
              onPick={openPicker}
              onRemove={removeMedia}
            />
          )}
          {selectedModel?.parameters.supports_seed && (
            <div className='border-border/70 border-t px-2.5 py-2'>
              <div className='grid max-w-40 gap-1.5'>
                <Label htmlFor='video-seed'>{t('Seed')}</Label>
                <Input
                  id='video-seed'
                  inputMode='numeric'
                  min={0}
                  onChange={(event) =>
                    updateConfig({ seed: event.target.value })
                  }
                  placeholder={t('Random')}
                  type='number'
                  value={effectiveConfig.seed}
                />
              </div>
            </div>
          )}
          <div className='border-border/70 grid gap-2 border-t p-2.5 sm:grid-cols-2 xl:grid-cols-[1.2fr_1.3fr_1fr_.75fr_.7fr_.7fr]'>
            <GroupedSelectField
              groups={catalogSelectGroups(availableCatalog)}
              hideLabel
              label={t('Model category')}
              value={selectedCatalogItem?.id ?? ''}
              onChange={handleCatalogChange}
            />
            <SelectField
              hideLabel
              label={t('Specific model')}
              value={selectedModel?.model ?? ''}
              options={(selectedCatalogItem?.models ?? []).map((model) => ({
                label: model.model,
                value: model.model,
              }))}
              onChange={handleModelChange}
            />
            <SelectField
              hideLabel
              label={t('Generation mode')}
              value={effectiveConfig.mode}
              options={(selectedModel?.modes ?? ['text']).map((mode) => ({
                label: videoModeLabel(mode, t),
                value: mode,
              }))}
              onChange={(mode) => handleModeChange(normalizeMode(mode))}
            />
            <SelectField
              disabled={!selectedModel?.parameters.aspect_ratios?.length}
              hideLabel
              label={t('Aspect ratio')}
              value={effectiveConfig.aspectRatio}
              options={(selectedModel?.parameters.aspect_ratios ?? []).map(
                (value) => ({ label: value, value })
              )}
              onChange={(aspectRatio) => updateConfig({ aspectRatio })}
            />
            <SelectField
              disabled={!selectedModel?.parameters.durations?.length}
              hideLabel
              label={t('Duration')}
              value={String(effectiveConfig.duration)}
              options={(selectedModel?.parameters.durations ?? []).map(
                (value) => ({
                  label: localizedDuration(value, t),
                  value: String(value),
                })
              )}
              onChange={(value) => updateConfig({ duration: Number(value) })}
            />
            <SelectField
              disabled={!selectedModel?.parameters.resolutions?.length}
              hideLabel
              label={t('Resolution')}
              value={effectiveConfig.resolution}
              options={(selectedModel?.parameters.resolutions ?? []).map(
                (value) => ({ label: value, value })
              )}
              onChange={(resolution) => updateConfig({ resolution })}
            />
          </div>
          <div className='flex items-center justify-between gap-3 px-2.5 pb-2.5'>
            <div className='text-muted-foreground min-w-0 truncate text-xs'>
              {selectedModel?.model ?? t('No model')}
              {materialCount > 0
                ? ' · ' + String(materialCount) + ' ' + t('media')
                : ''}
            </div>
            <Button
              disabled={submitDisabled}
              onClick={() => void submit()}
              type='button'
            >
              {isSubmitting ? (
                <Loader2Icon className='animate-spin' />
              ) : (
                <SendIcon />
              )}
              {t('Generate')}
            </Button>
          </div>
        </div>
        <input
          className='hidden'
          onChange={(event) => handleFiles(event.target.files)}
          ref={fileInputRef}
          type='file'
        />
      </div>
    </div>
  )
}

function videoModeLabel(mode: VideoMode, t: (key: string) => string) {
  const labels: Record<VideoMode, string> = {
    text: t('Text to video'),
    first_frame: t('First frame to video'),
    first_last: t('First and last frame'),
    reference: t('Multi-reference video'),
    video_edit: t('Video editing'),
  }
  return labels[mode]
}

function localizedDuration(value: number, t: (key: string) => string) {
  return String(value) + ' ' + t('seconds')
}

function SelectField({
  className,
  disabled = false,
  hideLabel = false,
  label,
  value,
  options,
  onChange,
}: {
  className?: string
  disabled?: boolean
  hideLabel?: boolean
  label: string
  value: string
  options: SelectOption[]
  onChange: (value: string) => void
}) {
  return (
    <div className={cn('grid min-w-0 gap-1.5', className)}>
      {!hideLabel && <Label>{label}</Label>}
      <select
        aria-label={label}
        className='border-input bg-background focus-visible:border-ring focus-visible:ring-ring/50 h-9 w-full min-w-0 rounded-md border px-3 text-sm outline-none focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50'
        disabled={disabled}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  )
}

function GroupedSelectField({
  className,
  disabled = false,
  hideLabel = false,
  label,
  value,
  groups,
  onChange,
}: {
  className?: string
  disabled?: boolean
  hideLabel?: boolean
  label: string
  value: string
  groups: SelectOptionGroup[]
  onChange: (value: string) => void
}) {
  return (
    <div className={cn('grid min-w-0 gap-1.5', className)}>
      {!hideLabel && <Label>{label}</Label>}
      <select
        aria-label={label}
        className='border-input bg-background focus-visible:border-ring focus-visible:ring-ring/50 h-9 w-full min-w-0 rounded-md border px-3 text-sm outline-none focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50'
        disabled={disabled}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {groups.map((group) => (
          <optgroup key={group.label} label={group.label}>
            {group.options.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </div>
  )
}

function MaterialShelf({
  media,
  mode,
  model,
  note,
  onPick,
  onRemove,
}: {
  media: MediaState
  mode: VideoMode
  model: VideoModelCapability
  note: string
  onPick: (kind: MediaKind) => void
  onRemove: (kind: MediaKind, id: string) => void
}) {
  const { t } = useTranslation()
  const labels: Record<MediaKind, string> = {
    first: t('First frame'),
    last: t('Last frame'),
    images: t('Reference images'),
    videos: t('Reference videos'),
    audios: t('Reference audio'),
  }
  return (
    <div className='border-border grid gap-3 border-y px-4 py-3'>
      <div className='flex items-center justify-between gap-3'>
        <div className='min-w-0'>
          <h2 className='text-sm font-semibold'>{t('Reference media')}</h2>
          {note && (
            <p
              className='text-muted-foreground mt-1 truncate text-xs'
              title={note}
            >
              {note}
            </p>
          )}
        </div>
        <span className='text-muted-foreground text-xs'>
          {String(
            allowedKinds(mode).reduce(
              (total, kind) => total + media[kind].length,
              0
            )
          )}{' '}
          {t('selected')}
        </span>
      </div>
      <div className='flex flex-wrap items-start gap-2'>
        {allowedKinds(mode)
          .filter((kind) => maxReferences(model, kind) > 0)
          .map((kind) => (
            <div className='grid min-w-0 gap-1.5' key={kind}>
              <div className='flex items-center justify-between gap-2'>
                <Label className='text-xs'>{labels[kind]}</Label>
                <span className='text-muted-foreground text-[11px]'>
                  {String(media[kind].length)} /{' '}
                  {String(maxReferences(model, kind))}
                </span>
              </div>
              <div className='flex min-h-20 gap-2 overflow-x-auto pb-1'>
                {media[kind].map((item) => (
                  <MediaPreview
                    item={item}
                    key={item.id}
                    kind={kind}
                    label={labels[kind]}
                    onRemove={() => onRemove(kind, item.id)}
                  />
                ))}
                {media[kind].length < maxReferences(model, kind) && (
                  <button
                    className='border-border text-muted-foreground hover:bg-muted/60 hover:text-foreground flex h-20 w-20 shrink-0 flex-col items-center justify-center gap-1 rounded-md border border-dashed text-xs transition-colors'
                    onClick={() => onPick(kind)}
                    type='button'
                  >
                    {kind === 'videos' ? (
                      <FileVideoIcon className='size-4' />
                    ) : kind === 'audios' ? (
                      <FileAudioIcon className='size-4' />
                    ) : (
                      <ImagePlusIcon className='size-4' />
                    )}
                    <span className='max-w-[4.5rem] truncate'>
                      {labels[kind]}
                    </span>
                    <PlusIcon className='size-3' />
                  </button>
                )}
              </div>
            </div>
          ))}
      </div>
    </div>
  )
}

function MediaPreview({
  item,
  kind,
  label,
  onRemove,
}: {
  item: SelectedMedia
  kind: MediaKind
  label: string
  onRemove: () => void
}) {
  const { t } = useTranslation()
  return (
    <div
      className={cn(
        'border-border relative h-20 shrink-0 overflow-hidden rounded-md border bg-black',
        kind === 'audios' ? 'bg-muted/40 w-52' : 'w-28'
      )}
    >
      {kind === 'videos' ? (
        <video
          className='size-full object-cover'
          muted
          playsInline
          preload='metadata'
          src={item.previewUrl}
        />
      ) : kind === 'audios' ? (
        <div className='flex size-full items-center gap-2 px-2 pt-5'>
          <FileAudioIcon className='text-muted-foreground size-5 shrink-0' />
          <audio
            className='h-8 min-w-0 flex-1'
            controls
            src={item.previewUrl}
          />
        </div>
      ) : (
        <img
          alt={label}
          className='size-full object-cover'
          src={item.previewUrl}
        />
      )}
      <button
        aria-label={t('Remove')}
        className='bg-background/90 hover:bg-background absolute top-1 right-1 flex size-6 items-center justify-center rounded-md'
        onClick={onRemove}
        title={t('Remove')}
        type='button'
      >
        <XIcon className='size-3.5' />
      </button>
      <div className='bg-background/90 absolute right-1 bottom-1 left-1 truncate rounded px-1 py-0.5 text-[10px]'>
        {item.file.name}
      </div>
    </div>
  )
}

function EmptyCanvas({
  icon,
  title,
}: {
  icon: React.ReactNode
  title: string
}) {
  return (
    <div className='border-border bg-muted/20 flex min-h-72 items-center justify-center rounded-lg border border-dashed p-8 text-center'>
      <div className='grid gap-2'>
        <div className='text-muted-foreground mx-auto [&>svg]:size-9'>
          {icon}
        </div>
        <div className='text-sm font-medium'>{title}</div>
      </div>
    </div>
  )
}

function VideoHistoryCard({
  item,
  onRetry,
}: {
  item: VideoHistoryItem
  onRetry: () => void
}) {
  const { t } = useTranslation()
  const statusLabels: Record<VideoTaskStatus, string> = {
    queued: t('Queued'),
    in_progress: t('Generating'),
    completed: t('Completed'),
    failed: t('Failed'),
    timeout: t('Timed out'),
  }
  const failed = item.status === 'failed' || item.status === 'timeout'

  return (
    <article className='border-border bg-card overflow-hidden rounded-lg border'>
      <div className='bg-muted/20 relative aspect-video overflow-hidden'>
        {item.status === 'completed' && item.resultUrl ? (
          <video
            className='size-full bg-black object-contain'
            controls
            playsInline
            preload='metadata'
            src={item.resultUrl}
          />
        ) : (
          <div className='flex size-full items-center justify-center p-6'>
            <div className='grid max-w-sm gap-3 text-center'>
              {failed ? (
                <CircleAlertIcon className='text-destructive mx-auto size-8' />
              ) : (
                <Loader2Icon className='text-muted-foreground mx-auto size-8 animate-spin' />
              )}
              <div className='text-sm font-medium'>
                {statusLabels[item.status]}
              </div>
              {item.error && (
                <p className='text-muted-foreground line-clamp-3 text-xs'>
                  {item.error}
                </p>
              )}
              {isActiveTask(item) && (
                <Progress className='w-48' value={item.progress} />
              )}
            </div>
          </div>
        )}
      </div>
      <div className='grid gap-3 p-3'>
        <div className='flex min-w-0 items-start justify-between gap-3'>
          <div className='min-w-0'>
            <p className='line-clamp-2 text-sm font-medium'>{item.prompt}</p>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {item.model} · {item.group}
              {item.sourceNames?.length
                ? ' · ' + String(item.sourceNames.length) + ' ' + t('media')
                : ''}
            </p>
            {item.sourceNames?.length ? (
              <p
                className='text-muted-foreground mt-1 truncate text-[11px]'
                title={item.sourceNames.join(', ')}
              >
                {item.sourceNames.join(', ')}
              </p>
            ) : null}
          </div>
          <span className='bg-muted shrink-0 rounded-md px-2 py-1 text-[11px] font-medium'>
            {statusLabels[item.status]}
          </span>
        </div>
        <div className='flex items-center justify-between gap-3'>
          <div className='text-muted-foreground truncate text-xs'>
            {new Date(item.createdAt).toLocaleString()}
          </div>
          <div className='flex items-center gap-1'>
            {item.resultUrl && (
              <>
                <Button
                  aria-label={t('Open')}
                  render={
                    <a
                      href={item.resultUrl}
                      rel='noopener noreferrer'
                      target='_blank'
                    />
                  }
                  size='icon-sm'
                  title={t('Open')}
                  variant='ghost'
                >
                  <ExternalLinkIcon />
                </Button>
                <Button
                  aria-label={t('Download')}
                  render={<a download href={item.resultUrl} />}
                  size='icon-sm'
                  title={t('Download')}
                  variant='ghost'
                >
                  <DownloadIcon />
                </Button>
              </>
            )}
            {failed && (
              <Button
                aria-label={t('Retry')}
                onClick={onRetry}
                size='icon-sm'
                title={t('Retry')}
                type='button'
                variant='ghost'
              >
                <RefreshCwIcon />
              </Button>
            )}
          </div>
        </div>
      </div>
    </article>
  )
}
