import type { VideoModelCapability, VideoStudioConfig } from './types'

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

export function requestSize(
  model: VideoModelCapability,
  config: VideoStudioConfig
) {
  if (model.profile === 'ali' && config.mode === 'text') {
    return ratioToPixelSize(config.aspectRatio).replace('x', '*')
  }
  if (
    (model.profile === 'sora' || model.profile === 'openai') &&
    /^sora(?:[-_.]|$)/i.test(model.model)
  ) {
    if (config.resolution === '1080p') {
      return config.aspectRatio === '9:16' ? '1024x1792' : '1792x1024'
    }
    return config.aspectRatio === '9:16' ? '720x1280' : '1280x720'
  }
  return model.parameters.resolutions?.length
    ? config.resolution
    : ratioToPixelSize(config.aspectRatio)
}

export function resolveConfigForModel(
  config: VideoStudioConfig,
  group: string,
  model: VideoModelCapability
): VideoStudioConfig {
  const durations = model.parameters.durations ?? []
  const aspectRatios = model.parameters.aspect_ratios ?? []
  const resolutions = model.parameters.resolutions ?? []
  const currentMode = config.mode
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
