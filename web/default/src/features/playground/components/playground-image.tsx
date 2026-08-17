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
import { useEffect, useMemo, useState } from 'react'
import type { FileUIPart } from 'ai'
import {
  DownloadIcon,
  ExternalLinkIcon,
  ImageIcon,
  ImagePlusIcon,
  Loader2Icon,
  PaperclipIcon,
  SendIcon,
  Trash2Icon,
} from 'lucide-react'
import { nanoid } from 'nanoid'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  PromptInput,
  PromptInputAttachment,
  PromptInputAttachments,
  PromptInputButton,
  PromptInputFooter,
  PromptInputProvider,
  PromptInputTextarea,
  PromptInputTools,
  usePromptInputAttachments,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { editImage, generateImage, sendChatCompletion } from '../api'
import {
  GEMINI_ASPECT_RATIO_OPTIONS,
  GEMINI_IMAGE_SIZE_OPTIONS,
  GPT_IMAGE_2_CUSTOM_SIZE_DEFAULT,
  GPT_IMAGE_2_SIZE_LIMITS,
  GPT_IMAGE_2_SIZE_OPTIONS,
  IMAGE_QUALITY_OPTIONS,
  IMAGE_SIZE_OPTIONS,
} from '../constants'
import type {
  GeminiImageSize,
  ImageGroupCapability,
  ImageHistoryItem,
  ImagePlaygroundConfig,
  ImageQuality,
  ImageResult,
  ModelOption,
} from '../types'

interface PlaygroundImageProps {
  config: ImagePlaygroundConfig
  history: ImageHistoryItem[]
  capabilities: ImageGroupCapability[]
  isModelLoading?: boolean
  onConfigChange: <K extends keyof ImagePlaygroundConfig>(
    key: K,
    value: ImagePlaygroundConfig[K]
  ) => void
  onConfigChangeValues: (values: Partial<ImagePlaygroundConfig>) => void
  onHistoryChange: (
    updater:
      | ImageHistoryItem[]
      | ((prev: ImageHistoryItem[]) => ImageHistoryItem[])
  ) => void
  onClearHistory: () => void
}

function isGptImage2Model(model: string) {
  const value = model.toLowerCase()
  return value === 'gpt-image-2' || value.startsWith('gpt-image-2-')
}

function normalizeImageSize(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, '')
}

function parseImageSize(value: string) {
  const match = normalizeImageSize(value).match(/^(\d{2,5})x(\d{2,5})$/)
  if (!match) return null
  const width = Number(match[1])
  const height = Number(match[2])
  if (!Number.isFinite(width) || !Number.isFinite(height)) return null
  return { width, height, normalized: `${width}x${height}` }
}

function validateGptImage2Size(size: string, t: (key: string) => string) {
  if (size === 'auto') return null

  const parsed = parseImageSize(size)
  if (!parsed) {
    return t('Use WIDTHxHEIGHT, for example 3840x2160')
  }

  const { width, height } = parsed
  const {
    MIN_DIMENSION,
    MAX_DIMENSION,
    MIN_PIXELS,
    MAX_PIXELS,
    MAX_ASPECT_RATIO,
  } = GPT_IMAGE_2_SIZE_LIMITS

  if (width < MIN_DIMENSION || height < MIN_DIMENSION) {
    return t('Width and height must be at least 16 px')
  }
  if (width > MAX_DIMENSION || height > MAX_DIMENSION) {
    return t('Width and height must be 3840 px or less')
  }
  if (width % 16 !== 0 || height % 16 !== 0) {
    return t('Width and height must be multiples of 16')
  }
  if (width * height > MAX_PIXELS) {
    return t('Total pixels must be 8,294,400 or less')
  }
  if (width * height < MIN_PIXELS) {
    return t('Total pixels must be at least 655,360')
  }
  if (Math.max(width, height) / Math.min(width, height) > MAX_ASPECT_RATIO) {
    return t('Aspect ratio must be 3:1 or less')
  }

  return null
}

function imageSrc(result: ImageResult) {
  if (result.dataUrl) return result.dataUrl
  if (result.b64Json) return `data:image/png;base64,${result.b64Json}`
  return result.url ?? ''
}

function blobToDataUrl(blob: Blob) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () =>
      reject(reader.error ?? new Error('Failed to read image'))
    reader.onload = () => resolve(String(reader.result || ''))
    reader.readAsDataURL(blob)
  })
}

async function filePartToDataUrl(file: FileUIPart) {
  if (file.url?.startsWith('data:')) return file.url
  return blobToDataUrl(await filePartToBlob(file))
}

function extractMarkdownImageDataUrls(content: string) {
  const results: ImageResult[] = []
  const pattern = /!\[[^\]]*\]\((data:image\/[^)]+)\)/g
  for (const match of content.matchAll(pattern)) {
    if (match[1]) results.push({ dataUrl: match[1] })
  }
  return results
}

function dataUrlToBlob(dataUrl: string) {
  const [meta = '', data = ''] = dataUrl.split(',')
  const mediaType = meta.match(/data:(.*?);base64/)?.[1] || 'image/png'
  const binary = atob(data)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return new Blob([bytes], { type: mediaType })
}

async function filePartToBlob(file: FileUIPart) {
  if (!file.url) {
    throw new Error('Attachment is missing image data')
  }
  if (file.url.startsWith('data:')) {
    return dataUrlToBlob(file.url)
  }
  const response = await fetch(file.url)
  return response.blob()
}

function downloadUrl(url: string, filename: string) {
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

function openUrl(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer')
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
    err?.response?.data?.error?.message ||
    err?.response?.data?.message ||
    err?.message ||
    fallback
  )
}

export function PlaygroundImage({
  config,
  history,
  capabilities,
  isModelLoading = false,
  onConfigChange,
  onConfigChangeValues,
  onHistoryChange,
  onClearHistory,
}: PlaygroundImageProps) {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const selectedGroupCapability = useMemo(
    () =>
      capabilities.find((group) => group.group === config.group) ??
      capabilities[0],
    [capabilities, config.group]
  )
  const selectedModelCapability = useMemo(
    () =>
      selectedGroupCapability?.models.find(
        (model) => model.model === config.model
      ) ?? selectedGroupCapability?.models[0],
    [config.model, selectedGroupCapability]
  )
  const imageModels: ModelOption[] = useMemo(
    () =>
      selectedGroupCapability?.models.map((model) => ({
        label: model.model,
        value: model.model,
      })) ?? [],
    [selectedGroupCapability]
  )
  const imageGroups = useMemo(
    () =>
      capabilities.map((group) => ({
        label: group.group,
        value: group.group,
        ratio: typeof group.ratio === 'number' ? group.ratio : 0,
        desc: group.desc,
      })),
    [capabilities]
  )
  const hasImageModels = capabilities.some((group) => group.models.length > 0)
  const resolvedModel = selectedModelCapability?.model ?? ''
  const resolvedGroup = selectedGroupCapability?.group ?? ''
  const isUsingGeminiImage = selectedModelCapability?.protocol === 'gemini_chat'
  const isUsingGptImage2 = selectedModelCapability?.profile === 'gpt_image_2'
  const effectiveImageSize =
    selectedModelCapability?.fixed_image_size ?? config.imageSize
  const sizeError = isUsingGptImage2
    ? validateGptImage2Size(config.size, t)
    : null
  const requestSize =
    config.size === 'auto'
      ? 'auto'
      : (parseImageSize(config.size)?.normalized ?? config.size)
  const submitDisabled =
    isSubmitting ||
    !prompt.trim() ||
    !resolvedModel ||
    !resolvedGroup ||
    !hasImageModels ||
    isModelLoading ||
    !!sizeError

  useEffect(() => {
    if (!resolvedModel || isUsingGptImage2 || isUsingGeminiImage) return
    if (
      !IMAGE_SIZE_OPTIONS.includes(
        config.size as (typeof IMAGE_SIZE_OPTIONS)[number]
      )
    ) {
      onConfigChange('size', 'auto')
    }
  }, [
    config.size,
    isUsingGeminiImage,
    isUsingGptImage2,
    onConfigChange,
    resolvedModel,
  ])

  useEffect(() => {
    if (isUsingGeminiImage && config.n > 4) {
      onConfigChange('n', 4)
    }
  }, [config.n, isUsingGeminiImage, onConfigChange])

  const handleGroupChange = (groupName: string) => {
    const group = capabilities.find((item) => item.group === groupName)
    if (!group) return
    const model =
      group.models.find((item) => item.model === config.model) ??
      group.models[0]
    if (!model) return
    onConfigChangeValues({ group: group.group, model: model.model })
  }

  const handleModeChange = (mode: ImagePlaygroundConfig['mode']) => {
    onConfigChange('mode', mode)
  }

  const handleSubmit = async (message: PromptInputMessage) => {
    const text = message.text?.trim() || prompt.trim()
    if (!text || submitDisabled) return

    if (
      config.mode === 'edit' &&
      (!message.files || message.files.length === 0)
    ) {
      toast.error(t('Please attach at least one source image'))
      return
    }
    if (
      isUsingGeminiImage &&
      config.mode === 'edit' &&
      (message.files?.length ?? 0) > 14
    ) {
      toast.error(t('Nano Banana supports up to 14 source images'))
      return
    }

    setIsSubmitting(true)
    try {
      const basePayload = {
        model: resolvedModel,
        group: resolvedGroup,
        prompt: text,
        size: requestSize,
        quality: config.quality,
        n: config.n,
      }

      const files = message.files ?? []
      const sourceImages = await Promise.all(
        files.map(async (file) => ({
          name: file.filename || 'image.png',
          mediaType: file.mediaType,
          dataUrl: await filePartToDataUrl(file),
        }))
      )

      let results: ImageResult[] = []
      const requestedCount = isUsingGeminiImage
        ? Math.min(config.n, 4)
        : config.n

      if (isUsingGeminiImage) {
        let lastError: unknown
        for (
          let requestIndex = 0;
          requestIndex < requestedCount && results.length < requestedCount;
          requestIndex += 1
        ) {
          try {
            const content = [
              { type: 'text' as const, text },
              ...sourceImages.map((source) => ({
                type: 'image_url' as const,
                image_url: { url: source.dataUrl },
              })),
            ]
            const response = await sendChatCompletion({
              model: resolvedModel,
              group: resolvedGroup,
              stream: false,
              messages: [{ role: 'user', content }],
              extra_body: {
                google: {
                  image_config: {
                    aspect_ratio: config.aspectRatio,
                    image_size: effectiveImageSize,
                  },
                },
              },
            })
            const requestResults = response.choices.flatMap((choice) =>
              extractMarkdownImageDataUrls(choice.message?.content || '')
            )
            if (requestResults.length === 0) {
              throw new Error(t('No image returned'))
            }
            results.push(
              ...requestResults.slice(0, requestedCount - results.length)
            )
          } catch (error) {
            lastError = error
            break
          }
        }
        if (results.length === 0 && lastError) throw lastError
      } else {
        const response =
          config.mode === 'edit'
            ? await (async () => {
                const formData = new FormData()
                formData.append('model', basePayload.model)
                formData.append('group', basePayload.group)
                formData.append('prompt', basePayload.prompt)
                formData.append('size', basePayload.size)
                formData.append('quality', basePayload.quality)
                formData.append('n', String(basePayload.n))

                for (const file of files) {
                  const blob = await filePartToBlob(file)
                  formData.append(
                    'image',
                    blob,
                    file.filename || `source-${nanoid(6)}.png`
                  )
                }

                return editImage(formData)
              })()
            : await generateImage(basePayload)

        results = (response.data || []).map((item) => ({
          b64Json: item.b64_json,
          url: item.url,
          revisedPrompt: item.revised_prompt,
        }))
      }

      if (results.length === 0) {
        toast.error(t('No image returned'))
        return
      }

      const historyItem: ImageHistoryItem = {
        id: nanoid(),
        mode: config.mode,
        prompt: text,
        model: basePayload.model,
        group: basePayload.group,
        protocol: selectedModelCapability?.protocol,
        profile: selectedModelCapability?.profile,
        size: isUsingGeminiImage ? undefined : basePayload.size,
        quality: isUsingGeminiImage ? undefined : basePayload.quality,
        aspectRatio: isUsingGeminiImage ? config.aspectRatio : undefined,
        imageSize: isUsingGeminiImage ? effectiveImageSize : undefined,
        n: requestedCount,
        createdAt: Date.now(),
        sourceImages: config.mode === 'edit' ? sourceImages : undefined,
        results,
      }

      onHistoryChange((prev) => [historyItem, ...prev])
      setPrompt('')
      if (results.length < requestedCount) {
        toast.warning(
          t('Generated {{count}} of {{requested}} images', {
            count: results.length,
            requested: requestedCount,
          })
        )
      } else {
        toast.success(t('Image generated'))
      }
    } catch (error) {
      toast.error(getErrorMessage(error, t('Image request failed')))
      throw error
    } finally {
      setIsSubmitting(false)
    }
  }

  const latestRevisedPrompt = history.find((item) =>
    item.results.some((result) => result.revisedPrompt)
  )

  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
      <div className='flex-1 overflow-y-auto'>
        <div className='mx-auto flex w-full max-w-5xl flex-col gap-4 px-4 py-4'>
          <div className='grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-start'>
            <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
              <SegmentedMode
                mode={config.mode}
                onModeChange={handleModeChange}
              />
              {isUsingGeminiImage ? (
                <>
                  <SelectField
                    label={t('Aspect ratio')}
                    value={config.aspectRatio}
                    onValueChange={(value) =>
                      onConfigChange('aspectRatio', value)
                    }
                    options={GEMINI_ASPECT_RATIO_OPTIONS.map((value) => ({
                      label: value,
                      value,
                    }))}
                  />
                  <SelectField
                    disabled={!!selectedModelCapability?.fixed_image_size}
                    label={
                      selectedModelCapability?.fixed_image_size
                        ? `${t('Resolution')} (${t('Locked')})`
                        : t('Resolution')
                    }
                    value={effectiveImageSize}
                    onValueChange={(value) =>
                      onConfigChange('imageSize', value as GeminiImageSize)
                    }
                    options={GEMINI_IMAGE_SIZE_OPTIONS.map((value) => ({
                      label: value,
                      value,
                    }))}
                  />
                </>
              ) : (
                <>
                  <ImageSizeField
                    value={config.size}
                    model={resolvedModel}
                    error={sizeError}
                    onValueChange={(value) => onConfigChange('size', value)}
                  />
                  <SelectField
                    label={t('Quality')}
                    value={config.quality}
                    onValueChange={(value) =>
                      onConfigChange('quality', value as ImageQuality)
                    }
                    options={IMAGE_QUALITY_OPTIONS.map((value) => ({
                      label: value,
                      value,
                    }))}
                  />
                </>
              )}
              <SelectField
                label={t('Count')}
                value={String(config.n)}
                onValueChange={(value) => onConfigChange('n', Number(value))}
                options={Array.from(
                  { length: isUsingGptImage2 ? 10 : 4 },
                  (_, index) => index + 1
                ).map((value) => ({
                  label: String(value),
                  value: String(value),
                }))}
              />
            </div>
            <Button
              className='w-fit'
              disabled={history.length === 0 || isSubmitting}
              onClick={onClearHistory}
              size='sm'
              type='button'
              variant='outline'
            >
              <Trash2Icon className='size-4' />
              <span>{t('Clear')}</span>
            </Button>
          </div>

          {!hasImageModels && !isModelLoading ? (
            <div className='border-border bg-muted/20 flex min-h-64 items-center justify-center rounded-lg border border-dashed p-8 text-center'>
              <div className='grid max-w-sm gap-2'>
                <ImageIcon className='text-muted-foreground mx-auto size-8' />
                <div className='text-sm font-medium'>
                  {t('No image models available')}
                </div>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Add an image model to a usable channel to enable generation.'
                  )}
                </p>
              </div>
            </div>
          ) : history.length === 0 ? (
            <div className='border-border bg-muted/20 flex min-h-64 items-center justify-center rounded-lg border border-dashed p-8 text-center'>
              <div className='grid max-w-sm gap-2'>
                <ImagePlusIcon className='text-muted-foreground mx-auto size-8' />
                <div className='text-sm font-medium'>
                  {t('Images will appear here')}
                </div>
              </div>
            </div>
          ) : (
            <div className='grid gap-4'>
              {history.map((item) => (
                <ImageHistoryCard key={item.id} item={item} />
              ))}
            </div>
          )}

          {latestRevisedPrompt && (
            <p className='text-muted-foreground text-xs'>
              {t('Latest revised prompt')}:{' '}
              {
                latestRevisedPrompt.results.find(
                  (result) => result.revisedPrompt
                )?.revisedPrompt
              }
            </p>
          )}
        </div>
      </div>

      <div className='mx-auto w-full max-w-4xl px-1 md:pb-4'>
        <PromptInputProvider>
          <PromptInput
            accept='image/*'
            groupClassName='rounded-xl'
            multiple
            onError={(error) => toast.error(error.message)}
            onSubmit={handleSubmit}
          >
            <PromptInputTextarea
              autoComplete='off'
              autoCorrect='off'
              autoCapitalize='off'
              spellCheck={false}
              className='px-5 md:text-base'
              disabled={isSubmitting || !hasImageModels}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={
                config.mode === 'edit'
                  ? t('Describe the image edit')
                  : t('Describe the image to generate')
              }
              value={prompt}
            />
            {config.mode === 'edit' && (
              <div className='flex flex-wrap gap-2 px-3 pb-2'>
                <PromptInputAttachments>
                  {(attachment) => <PromptInputAttachment data={attachment} />}
                </PromptInputAttachments>
              </div>
            )}
            <PromptInputFooter className='p-2.5'>
              <PromptInputTools>
                {config.mode === 'edit' && (
                  <SourceImageButton disabled={isSubmitting} />
                )}
              </PromptInputTools>
              <div className='flex items-center gap-1.5 md:gap-2'>
                <ModelGroupSelector
                  selectedModel={resolvedModel}
                  models={imageModels}
                  onModelChange={(value) => onConfigChange('model', value)}
                  selectedGroup={resolvedGroup}
                  groups={imageGroups}
                  onGroupChange={handleGroupChange}
                  disabled={
                    isSubmitting ||
                    isModelLoading ||
                    imageModels.length === 0 ||
                    imageGroups.length === 0
                  }
                />
                <PromptInputButton
                  className='text-foreground font-medium'
                  disabled={submitDisabled}
                  type='submit'
                  variant='secondary'
                >
                  {isSubmitting ? (
                    <Loader2Icon className='animate-spin' size={16} />
                  ) : (
                    <SendIcon size={16} />
                  )}
                  <span className='hidden sm:inline'>
                    {config.mode === 'edit' ? t('Edit') : t('Generate')}
                  </span>
                  <span className='sr-only sm:hidden'>
                    {config.mode === 'edit' ? t('Edit') : t('Generate')}
                  </span>
                </PromptInputButton>
              </div>
            </PromptInputFooter>
          </PromptInput>
        </PromptInputProvider>
      </div>
    </div>
  )
}

function SourceImageButton({ disabled }: { disabled?: boolean }) {
  const { t } = useTranslation()
  const attachments = usePromptInputAttachments()
  return (
    <PromptInputButton
      className='border font-medium'
      disabled={disabled}
      onClick={() => attachments.openFileDialog()}
      type='button'
      variant='outline'
    >
      <PaperclipIcon size={16} />
      <span className='hidden sm:inline'>{t('Source image')}</span>
      <span className='sr-only sm:hidden'>{t('Source image')}</span>
    </PromptInputButton>
  )
}

function SegmentedMode({
  mode,
  onModeChange,
}: {
  mode: ImagePlaygroundConfig['mode']
  onModeChange: (mode: ImagePlaygroundConfig['mode']) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='grid gap-1.5'>
      <Label>{t('Mode')}</Label>
      <div className='bg-muted grid h-8 grid-cols-2 rounded-lg p-[3px]'>
        {(['generate', 'edit'] as const).map((value) => (
          <button
            className={cn(
              'inline-flex items-center justify-center gap-1.5 rounded-md px-2 text-xs font-medium transition-colors',
              mode === value
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            )}
            key={value}
            onClick={() => onModeChange(value)}
            type='button'
          >
            {value === 'generate' ? (
              <ImagePlusIcon className='size-3.5' />
            ) : (
              <PaperclipIcon className='size-3.5' />
            )}
            <span>{value === 'generate' ? t('Generate') : t('Edit')}</span>
          </button>
        ))}
      </div>
    </div>
  )
}

function SelectField({
  label,
  value,
  onValueChange,
  options,
  disabled = false,
}: {
  label: string
  value: string
  onValueChange: (value: string) => void
  options: Array<{ label: string; value: string }>
  disabled?: boolean
}) {
  return (
    <div className='grid gap-1.5'>
      <Label>{label}</Label>
      <Select
        disabled={disabled}
        value={value}
        onValueChange={(nextValue) => {
          if (nextValue) onValueChange(nextValue)
        }}
      >
        <SelectTrigger className='w-full'>
          <SelectValue>
            {options.find((option) => option.value === value)?.label}
          </SelectValue>
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

function ImageSizeField({
  value,
  model,
  error,
  onValueChange,
}: {
  value: string
  model: string
  error?: string | null
  onValueChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const isGptImage2 = isGptImage2Model(model)
  const presetOptions = isGptImage2
    ? GPT_IMAGE_2_SIZE_OPTIONS
    : IMAGE_SIZE_OPTIONS
  const isPresetValue = (presetOptions as readonly string[]).includes(value)
  const selectedPreset = isPresetValue ? value : 'custom'
  const customValue =
    value !== 'auto' && !isPresetValue ? value : GPT_IMAGE_2_CUSTOM_SIZE_DEFAULT

  return (
    <div className='grid gap-1.5'>
      <Label>{t('Size')}</Label>
      <div className='grid gap-2'>
        <Select
          value={selectedPreset}
          onValueChange={(nextValue) => {
            if (!nextValue) return
            if (nextValue === 'custom') {
              onValueChange(customValue)
              return
            }
            onValueChange(nextValue)
          }}
        >
          <SelectTrigger className='w-full'>
            <SelectValue>
              {selectedPreset === 'custom' ? t('Custom') : selectedPreset}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {presetOptions.map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
            {isGptImage2 && (
              <SelectItem value='custom'>{t('Custom')}</SelectItem>
            )}
          </SelectContent>
        </Select>
        {isGptImage2 && selectedPreset === 'custom' && (
          <Input
            aria-invalid={!!error}
            inputMode='numeric'
            onChange={(event) =>
              onValueChange(normalizeImageSize(event.target.value))
            }
            placeholder='3840x2160'
            value={value}
          />
        )}
      </div>
      {isGptImage2 && selectedPreset === 'custom' ? (
        <p
          className={cn(
            'text-xs leading-5',
            error ? 'text-destructive' : 'text-muted-foreground'
          )}
        >
          {error ||
            t('Max 3840 px per side, multiples of 16, up to 8,294,400 pixels.')}
        </p>
      ) : null}
    </div>
  )
}

function ImageHistoryCard({ item }: { item: ImageHistoryItem }) {
  const { t } = useTranslation()
  return (
    <div className='border-border bg-background overflow-hidden rounded-lg border'>
      <div className='border-border flex flex-col gap-2 border-b p-3 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0 space-y-1'>
          <div className='flex flex-wrap items-center gap-2 text-xs'>
            <span className='bg-muted rounded-md px-1.5 py-0.5 font-medium'>
              {item.mode === 'edit' ? t('Edit') : t('Generate')}
            </span>
            <span className='text-muted-foreground'>{item.model}</span>
            {item.profile === 'gemini_image' ||
            item.protocol === 'gemini_chat' ? (
              <>
                <span className='text-muted-foreground'>
                  {item.aspectRatio}
                </span>
                <span className='text-muted-foreground'>{item.imageSize}</span>
              </>
            ) : (
              <>
                {item.size && (
                  <span className='text-muted-foreground'>{item.size}</span>
                )}
                {item.quality && (
                  <span className='text-muted-foreground'>{item.quality}</span>
                )}
              </>
            )}
          </div>
          <p className='text-sm leading-relaxed break-words'>{item.prompt}</p>
        </div>
        <span className='text-muted-foreground shrink-0 text-xs'>
          {new Date(item.createdAt).toLocaleString()}
        </span>
      </div>

      {!!item.sourceImages?.length && (
        <div className='border-border flex gap-2 border-b p-3'>
          {item.sourceImages.map((source) => (
            <img
              alt={source.name}
              className='border-border size-14 rounded-md border object-cover'
              key={`${item.id}-${source.name}`}
              src={source.dataUrl}
            />
          ))}
        </div>
      )}

      <div
        className={cn(
          'grid gap-3 p-3',
          item.results.length > 1 ? 'sm:grid-cols-2 lg:grid-cols-3' : ''
        )}
      >
        {item.results.map((result, index) => {
          const src = imageSrc(result)
          const filename = `${item.model}-${item.id}-${index + 1}.png`
          return (
            <div
              className='border-border bg-muted/20 overflow-hidden rounded-lg border'
              key={`${item.id}-${index}`}
            >
              {src ? (
                <img
                  alt={`${item.prompt} ${index + 1}`}
                  className='bg-background aspect-square w-full object-contain'
                  src={src}
                />
              ) : (
                <div className='flex aspect-square w-full items-center justify-center'>
                  <ImageIcon className='text-muted-foreground size-8' />
                </div>
              )}
              <div className='flex items-center justify-between gap-2 p-2'>
                <span className='text-muted-foreground text-xs'>
                  {t('Image')} {index + 1}
                </span>
                <div className='flex items-center gap-1'>
                  <Button
                    disabled={!src}
                    onClick={() => openUrl(src)}
                    size='icon-sm'
                    type='button'
                    variant='ghost'
                  >
                    <ExternalLinkIcon className='size-4' />
                    <span className='sr-only'>{t('Open')}</span>
                  </Button>
                  <Button
                    disabled={!src}
                    onClick={() => downloadUrl(src, filename)}
                    size='icon-sm'
                    type='button'
                    variant='ghost'
                  >
                    <DownloadIcon className='size-4' />
                    <span className='sr-only'>{t('Download')}</span>
                  </Button>
                </div>
              </div>
              {result.revisedPrompt && (
                <p className='text-muted-foreground border-border border-t px-2 py-2 text-xs leading-relaxed break-words'>
                  {result.revisedPrompt}
                </p>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
