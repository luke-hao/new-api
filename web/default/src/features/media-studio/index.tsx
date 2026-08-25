import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ClapperboardIcon, ImageIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getImageCapabilities } from '@/features/playground/api'
import { PlaygroundImage } from '@/features/playground/components/playground-image'
import { usePlaygroundState } from '@/features/playground/hooks'
import { getVideoCapabilities } from './api'
import { VideoStudio } from './video-studio'

export function MediaStudio() {
  const { t } = useTranslation()
  const {
    imageConfig,
    imageHistory,
    updateImageConfig,
    updateImageConfigValues,
    updateImageHistory,
    clearImageHistory,
  } = usePlaygroundState()

  const {
    data: imageCapabilities,
    error: imageCapabilitiesError,
    isLoading: isLoadingImages,
  } = useQuery({
    queryKey: ['playground-image-capabilities'],
    queryFn: getImageCapabilities,
  })

  const {
    data: videoCapabilities,
    error: videoCapabilitiesError,
    isLoading: isLoadingVideos,
  } = useQuery({
    queryKey: ['playground-video-capabilities'],
    queryFn: getVideoCapabilities,
  })

  useEffect(() => {
    if (!imageCapabilitiesError) return
    toast.error(
      imageCapabilitiesError instanceof Error
        ? imageCapabilitiesError.message
        : t('Failed to load image capabilities')
    )
  }, [imageCapabilitiesError, t])

  useEffect(() => {
    if (!videoCapabilitiesError) return
    toast.error(
      videoCapabilitiesError instanceof Error
        ? videoCapabilitiesError.message
        : t('Failed to load video capabilities')
    )
  }, [t, videoCapabilitiesError])

  useEffect(() => {
    if (!imageCapabilities?.length) return
    const group =
      imageCapabilities.find((item) => item.group === imageConfig.group) ??
      imageCapabilities[0]
    const model =
      group.models.find((item) => item.model === imageConfig.model) ??
      group.models[0]
    if (!model) return
    if (
      group.group !== imageConfig.group ||
      model.model !== imageConfig.model
    ) {
      updateImageConfigValues({ group: group.group, model: model.model })
    }
  }, [
    imageCapabilities,
    imageConfig.group,
    imageConfig.model,
    updateImageConfigValues,
  ])

  return (
    <Tabs
      defaultValue='image'
      className='relative flex size-full flex-col overflow-hidden'
    >
      <div className='border-border/60 bg-background/90 flex shrink-0 justify-center border-b px-4 py-2 backdrop-blur'>
        <TabsList>
          <TabsTrigger value='image'>
            <ImageIcon className='size-4' />
            {t('Image')}
          </TabsTrigger>
          <TabsTrigger value='video'>
            <ClapperboardIcon className='size-4' />
            {t('Video')}
          </TabsTrigger>
        </TabsList>
      </div>

      <TabsContent value='image' className='flex min-h-0 flex-1 flex-col'>
        <PlaygroundImage
          config={imageConfig}
          history={imageHistory}
          capabilities={imageCapabilities ?? []}
          isModelLoading={isLoadingImages}
          onConfigChange={updateImageConfig}
          onConfigChangeValues={updateImageConfigValues}
          onHistoryChange={updateImageHistory}
          onClearHistory={clearImageHistory}
        />
      </TabsContent>

      <TabsContent value='video' className='flex min-h-0 flex-1 flex-col'>
        <VideoStudio
          capabilities={videoCapabilities ?? []}
          isLoading={isLoadingVideos}
        />
      </TabsContent>
    </Tabs>
  )
}
