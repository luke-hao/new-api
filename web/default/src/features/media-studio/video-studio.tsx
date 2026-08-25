import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  CircleAlertIcon,
  ClapperboardIcon,
  DownloadIcon,
  ExternalLinkIcon,
  ImagePlusIcon,
  Loader2Icon,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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

const CONFIG_STORAGE_KEY = 'media_studio_video_config'
const HISTORY_STORAGE_KEY = 'media_studio_video_history'
const POLL_INTERVAL_MS = 5000
const TASK_TIMEOUT_MS = 30 * 60 * 1000
const MAX_HISTORY_ITEMS = 30

const DEFAULT_CONFIG: VideoStudioConfig = {
  group: 'default',
  model: '',
  mode: 'text',
  duration: 5,
  aspectRatio: '16:9',
  resolution: '720p',
  seed: '',
}

type SelectedFrame = {
  file: File
  previewUrl: string
}

type SelectOption = {
  label: string
  value: string
}

function loadStoredValue<T>(key: string, fallback: T): T {
  if (typeof window === 'undefined') return fallback
  try {
    const raw = window.localStorage.getItem(key)
    return raw ? ({ ...fallback, ...JSON.parse(raw) } as T) : fallback
  } catch {
    return fallback
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

function taskResponseUrl(response: VideoTaskResponse) {
  return response.url || response.metadata?.url || undefined
}

function ratioToPixelSize(aspectRatio: string) {
  const sizes: Record<string, string> = {
    '16:9': '1280x720',
    '9:16': '720x1280',
    '1:1': '1024x1024',
    '4:3': '1024x768',
    '3:4': '768x1024',
    '21:9': '1792x768',
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
  return {
    ...config,
    group,
    model: model.model,
    mode: model.modes.includes(config.mode)
      ? config.mode
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
  const progress = Math.max(
    0,
    Math.min(
      100,
      Number(response.progress ?? (status === 'completed' ? 100 : 0))
    )
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

export function VideoStudio({
  capabilities,
  isLoading = false,
}: {
  capabilities: VideoGroupCapability[]
  isLoading?: boolean
}) {
  const { t } = useTranslation()
  const [config, setConfig] = useState<VideoStudioConfig>(() =>
    loadStoredValue(CONFIG_STORAGE_KEY, DEFAULT_CONFIG)
  )
  const [history, setHistory] = useState<VideoHistoryItem[]>(loadStoredHistory)
  const [prompt, setPrompt] = useState('')
  const [frames, setFrames] = useState<SelectedFrame[]>([])
  const [isSubmitting, setIsSubmitting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const frameTargetRef = useRef(0)
  const framesRef = useRef(frames)
  const historyRef = useRef(history)

  const selectedGroup = useMemo(
    () =>
      capabilities.find((item) => item.group === config.group) ??
      capabilities[0],
    [capabilities, config.group]
  )
  const selectedModel = useMemo(
    () =>
      selectedGroup?.models.find((item) => item.model === config.model) ??
      selectedGroup?.models[0],
    [config.model, selectedGroup]
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
    framesRef.current = frames
  }, [frames])

  useEffect(() => {
    return () => {
      for (const frame of framesRef.current) {
        URL.revokeObjectURL(frame.previewUrl)
      }
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
          const response = await getVideoTask(item.taskId)
          return { id: item.id, response }
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

  const trimFramesForMode = useCallback((mode: VideoMode) => {
    const requiredFrames = mode === 'text' ? 0 : mode === 'image' ? 1 : 2
    setFrames((items) => {
      if (items.length <= requiredFrames) return items
      for (const frame of items.slice(requiredFrames)) {
        URL.revokeObjectURL(frame.previewUrl)
      }
      return items.slice(0, requiredFrames)
    })
  }, [])

  const applyModelConfig = useCallback(
    (groupName: string, model: VideoModelCapability) => {
      const next = resolveConfigForModel(effectiveConfig, groupName, model)
      updateConfig(next)
      trimFramesForMode(next.mode)
    },
    [effectiveConfig, trimFramesForMode, updateConfig]
  )

  const handleGroupChange = (groupName: string) => {
    const group = capabilities.find((item) => item.group === groupName)
    if (!group) return
    const model =
      group.models.find((item) => item.model === effectiveConfig.model) ??
      group.models[0]
    if (!model) return
    applyModelConfig(group.group, model)
  }

  const handleModelChange = (modelName: string) => {
    const model = selectedGroup?.models.find((item) => item.model === modelName)
    if (!selectedGroup || !model) return
    applyModelConfig(selectedGroup.group, model)
  }

  const handleModeChange = (mode: VideoMode) => {
    updateConfig({ mode })
    trimFramesForMode(mode)
  }

  const handleFrameFile = (file?: File) => {
    if (!file) return
    const target = frameTargetRef.current
    setFrames((previous) => {
      const next = [...previous]
      if (next[target]) URL.revokeObjectURL(next[target].previewUrl)
      next[target] = { file, previewUrl: URL.createObjectURL(file) }
      return next.filter(Boolean)
    })
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const removeFrame = (index: number) => {
    setFrames((items) => {
      if (items[index]) URL.revokeObjectURL(items[index].previewUrl)
      return items.filter((_, itemIndex) => itemIndex !== index)
    })
  }

  const openFramePicker = (index: number) => {
    frameTargetRef.current = index
    fileInputRef.current?.click()
  }

  const submit = async () => {
    if (!selectedGroup || !selectedModel || !prompt.trim()) return
    const requiredFrames =
      effectiveConfig.mode === 'text'
        ? 0
        : effectiveConfig.mode === 'image'
          ? 1
          : 2
    if (frames.length !== requiredFrames) {
      toast.error(
        effectiveConfig.mode === 'first_last'
          ? t('Add both first and last frames')
          : t('Add a source image')
      )
      return
    }

    const seedValue = effectiveConfig.seed.trim()
      ? Number(effectiveConfig.seed)
      : undefined
    const size = requestSize(selectedModel, effectiveConfig)
    const metadata: Record<string, unknown> = {
      aspect_ratio: effectiveConfig.aspectRatio,
      aspectRatio: effectiveConfig.aspectRatio,
      ratio: effectiveConfig.aspectRatio,
      resolution: effectiveConfig.resolution,
      seed: seedValue,
      parameters: {
        duration: effectiveConfig.duration,
        resolution: effectiveConfig.resolution,
        seed: seedValue,
      },
    }

    setIsSubmitting(true)
    try {
      let response: VideoTaskResponse
      if (requiredFrames === 0) {
        const payload: VideoSubmitPayload = {
          model: selectedModel.model,
          group: selectedGroup.group,
          prompt: prompt.trim(),
          duration: effectiveConfig.duration,
          seconds: String(effectiveConfig.duration),
          size,
          metadata,
        }
        response = await submitVideo(payload)
      } else {
        const form = new FormData()
        form.append('model', selectedModel.model)
        form.append('group', selectedGroup.group)
        form.append('prompt', prompt.trim())
        form.append('duration', String(effectiveConfig.duration))
        form.append('seconds', String(effectiveConfig.duration))
        form.append('size', size)
        form.append('metadata', JSON.stringify(metadata))
        for (const frame of frames) {
          form.append('input_reference', frame.file, frame.file.name)
        }
        response = await submitVideo(form)
      }

      const taskId = response.id || response.task_id
      if (!taskId) throw new Error(t('Video task ID was not returned'))

      const now = Date.now()
      const status = response.error
        ? 'failed'
        : normalizeStatus(response.status)
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
        sourceNames: frames.map((frame) => frame.file.name),
        status,
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
      mode: item.mode,
      duration: item.duration ?? config.duration,
      aspectRatio: item.aspectRatio ?? config.aspectRatio,
      resolution: item.resolution ?? config.resolution,
      seed: item.seed === undefined ? '' : String(item.seed),
    })
    setPrompt(item.prompt)
    setFrames((items) => {
      for (const frame of items) URL.revokeObjectURL(frame.previewUrl)
      return []
    })
    toast.info(
      item.mode === 'text'
        ? t('Ready to retry')
        : t('Reattach source frames to retry')
    )
  }

  const hasModels = capabilities.some((group) => group.models.length > 0)
  const requiredFrames =
    effectiveConfig.mode === 'text'
      ? 0
      : effectiveConfig.mode === 'image'
        ? 1
        : 2
  const submitDisabled =
    isSubmitting ||
    isLoading ||
    !hasModels ||
    !selectedModel ||
    !prompt.trim() ||
    frames.length !== requiredFrames

  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
      <div className='flex-1 overflow-y-auto'>
        <div className='mx-auto flex w-full max-w-6xl flex-col gap-4 px-4 py-4'>
          <div className='flex items-end justify-between gap-3'>
            <div className='grid min-w-0 flex-1 gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7'>
              <ModeControl
                modes={selectedModel?.modes ?? ['text']}
                value={effectiveConfig.mode}
                onChange={handleModeChange}
              />
              <SelectField
                label={t('Group')}
                value={selectedGroup?.group ?? ''}
                options={capabilities.map((group) => ({
                  label: group.group,
                  value: group.group,
                }))}
                onChange={handleGroupChange}
              />
              <SelectField
                className='sm:col-span-2 lg:col-span-1 xl:col-span-2'
                label={t('Model')}
                value={selectedModel?.model ?? ''}
                options={(selectedGroup?.models ?? []).map((model) => ({
                  label: model.model,
                  value: model.model,
                }))}
                onChange={handleModelChange}
              />
              {!!selectedModel?.parameters.durations?.length && (
                <SelectField
                  label={t('Duration')}
                  value={String(effectiveConfig.duration)}
                  options={selectedModel.parameters.durations.map((value) => ({
                    label: `${value}s`,
                    value: String(value),
                  }))}
                  onChange={(value) =>
                    updateConfig({ duration: Number(value) })
                  }
                />
              )}
              {!!selectedModel?.parameters.aspect_ratios?.length && (
                <SelectField
                  label={t('Aspect ratio')}
                  value={effectiveConfig.aspectRatio}
                  options={selectedModel.parameters.aspect_ratios.map(
                    (value) => ({ label: value, value })
                  )}
                  onChange={(aspectRatio) => updateConfig({ aspectRatio })}
                />
              )}
              {!!selectedModel?.parameters.resolutions?.length &&
                !(
                  selectedModel.profile === 'ali' &&
                  effectiveConfig.mode === 'text'
                ) && (
                  <SelectField
                    label={t('Resolution')}
                    value={effectiveConfig.resolution}
                    options={selectedModel.parameters.resolutions.map(
                      (value) => ({ label: value, value })
                    )}
                    onChange={(resolution) => updateConfig({ resolution })}
                  />
                )}
              {selectedModel?.parameters.supports_seed && (
                <div className='grid gap-1.5'>
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
              )}
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

      <div className='border-border/70 bg-background/95 shrink-0 border-t px-3 py-3 backdrop-blur md:px-4 md:pb-4'>
        <div className='border-border mx-auto w-full max-w-5xl overflow-hidden rounded-lg border'>
          {requiredFrames > 0 && (
            <div className='border-border/70 flex min-h-20 gap-2 overflow-x-auto border-b p-2'>
              {Array.from({ length: requiredFrames }, (_, index) => (
                <FrameSlot
                  frame={frames[index]}
                  index={index}
                  key={index}
                  mode={effectiveConfig.mode}
                  onPick={() => openFramePicker(index)}
                  onRemove={() => removeFrame(index)}
                />
              ))}
            </div>
          )}
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
          <div className='flex items-center justify-between gap-3 px-2.5 pb-2.5'>
            <div className='text-muted-foreground min-w-0 truncate text-xs'>
              {selectedModel?.model ?? t('No model')}
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
          accept='image/*'
          className='hidden'
          onChange={(event) => handleFrameFile(event.target.files?.[0])}
          ref={fileInputRef}
          type='file'
        />
      </div>
    </div>
  )
}

function ModeControl({
  modes,
  value,
  onChange,
}: {
  modes: VideoMode[]
  value: VideoMode
  onChange: (mode: VideoMode) => void
}) {
  const { t } = useTranslation()
  const labels: Record<VideoMode, string> = {
    text: t('Text to video'),
    image: t('Image to video'),
    first_last: t('First & last frame'),
  }
  return (
    <div className='grid gap-1.5 sm:col-span-2 lg:col-span-2'>
      <Label>{t('Mode')}</Label>
      <div
        className='bg-muted grid h-8 rounded-lg p-[3px]'
        style={{
          gridTemplateColumns: `repeat(${modes.length}, minmax(0, 1fr))`,
        }}
      >
        {modes.map((mode) => (
          <button
            className={cn(
              'min-w-0 truncate rounded-md px-2 text-xs font-medium transition-colors',
              mode === value
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            )}
            key={mode}
            onClick={() => onChange(mode)}
            title={labels[mode]}
            type='button'
          >
            {labels[mode]}
          </button>
        ))}
      </div>
    </div>
  )
}

function SelectField({
  className,
  label,
  value,
  options,
  onChange,
}: {
  className?: string
  label: string
  value: string
  options: SelectOption[]
  onChange: (value: string) => void
}) {
  return (
    <div className={cn('grid min-w-0 gap-1.5', className)}>
      <Label>{label}</Label>
      <Select value={value} onValueChange={(next) => onChange(next ?? '')}>
        <SelectTrigger className='w-full min-w-0'>
          <SelectValue className='min-w-0 truncate' />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

function FrameSlot({
  frame,
  index,
  mode,
  onPick,
  onRemove,
}: {
  frame?: SelectedFrame
  index: number
  mode: VideoMode
  onPick: () => void
  onRemove: () => void
}) {
  const { t } = useTranslation()
  const label =
    mode === 'first_last'
      ? index === 0
        ? t('First frame')
        : t('Last frame')
      : t('Source image')

  if (!frame) {
    return (
      <button
        className='border-border text-muted-foreground hover:bg-muted/60 hover:text-foreground flex h-16 w-28 shrink-0 items-center justify-center gap-1.5 rounded-lg border border-dashed text-xs transition-colors'
        onClick={onPick}
        type='button'
      >
        <ImagePlusIcon className='size-4' />
        {label}
      </button>
    )
  }

  return (
    <div className='border-border relative h-16 w-28 shrink-0 overflow-hidden rounded-lg border'>
      <img
        alt={label}
        className='size-full object-cover'
        src={frame.previewUrl}
      />
      <button
        aria-label={t('Remove')}
        className='bg-background/85 hover:bg-background absolute top-1 right-1 flex size-6 items-center justify-center rounded-md'
        onClick={onRemove}
        title={t('Remove')}
        type='button'
      >
        <XIcon className='size-3.5' />
      </button>
      <div className='bg-background/85 absolute right-1 bottom-1 left-1 truncate rounded px-1 py-0.5 text-[10px]'>
        {label}
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
            </p>
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
