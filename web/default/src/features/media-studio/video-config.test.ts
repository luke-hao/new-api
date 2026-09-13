// @ts-expect-error -- bun:test is provided by Bun and excluded from app types.
import { describe, expect, test } from 'bun:test'
import fixture from '../../../../../common/testdata/playground_video_aicopy.json'
import type {
  VideoMode,
  VideoModelCapability,
  VideoStudioConfig,
} from './types'
import {
  buildAvailableVideoCatalog,
  findAvailableVideoCatalogItem,
  flattenAvailableVideoCatalog,
} from './video-catalog'
import { requestSize, resolveConfigForModel } from './video-config'

const models: VideoModelCapability[] = fixture.map((row) => ({
  model: row.model,
  profile: 'openai',
  modes: row.modes as VideoMode[],
  parameters: {
    ...row,
    supports_seed: false,
    max_input_references: row.max_image_references,
    max_image_bytes: 15 << 20,
    max_video_bytes: 160 << 20,
    max_audio_bytes: 50 << 20,
    max_video_edit_bytes: 8 << 20,
  },
}))
const config: VideoStudioConfig = {
  group: '视频生成',
  model: '',
  mode: 'text',
  duration: 15,
  aspectRatio: '16:9',
  resolution: '720p',
  seed: '',
}
function getModel(name: string) {
  const model = models.find((m) => m.model === name)
  if (!model) throw Error(name)
  return model
}
describe('AICopy video controls', () => {
  test('all 45 models occur exactly once in the ten reviewed categories', () => {
    const sections = buildAvailableVideoCatalog(models)
    const all = flattenAvailableVideoCatalog(sections)
    expect(all).toHaveLength(45)
    expect(new Set(all.map((m) => m.model)).size).toBe(45)
    expect(
      sections.flatMap((s) => s.items).map((i) => [i.id, i.models.length])
    ).toEqual([
      ['happyhorse', 6],
      ['volcanoFutureH3', 3],
      ['volcengineArk', 2],
      ['volcengineSd20', 6],
      ['sd2', 7],
      ['sd900', 1],
      ['sd933', 2],
      ['sdPerUseAd', 10],
      ['omni', 4],
      ['grok15preview', 4],
    ])
    expect(buildAvailableVideoCatalog([])).toEqual([])
    for (const row of fixture)
      expect(findAvailableVideoCatalogItem(sections, row.model)?.id).toBe(
        row.category
      )
  })
  test('a saved model survives the merged category and invalid values normalize', () => {
    const model = getModel('【稳定】sd2.5-720p（按秒）')
    const result = resolveConfigForModel(
      { ...config, model: model.model, duration: 30 },
      '视频生成',
      model
    )
    expect(result.model).toBe(model.model)
    expect(result.duration).toBe(4)
    expect(resolveConfigForModel(config, '视频生成', model).duration).toBe(15)
  })
  test('every model normalizes stale controls and uses its selected resolution', () => {
    for (const model of models) {
      const next = resolveConfigForModel(
        {
          ...config,
          mode: 'video_edit',
          duration: 999,
          aspectRatio: 'invalid',
          resolution: 'invalid',
        },
        '视频生成',
        model
      )
      expect(model.modes).toContain(next.mode)
      expect(model.parameters.durations).toContain(next.duration)
      expect(model.parameters.aspect_ratios).toContain(next.aspectRatio)
      expect(model.parameters.resolutions).toContain(next.resolution)
      expect(requestSize(model, next)).toBe(next.resolution)
    }
  })
  test('Adobe fixes orientation, HappyHorse fixes mode, H3 requires references', () => {
    const adobe = resolveConfigForModel(
      config,
      '视频生成',
      getModel('sd2.0-1080满血-ad渠道9x16')
    )
    expect(adobe.aspectRatio).toBe('9:16')
    expect(adobe.resolution).toBe('1080p')
    expect(adobe.duration).toBe(15)
    const horse = resolveConfigForModel(
      config,
      '视频生成',
      getModel('happyhorse-1.1-i2v-1080p')
    )
    expect(horse.mode).toBe('first_frame')
    expect(horse.aspectRatio).toBe('跟随首帧')
    expect(
      resolveConfigForModel(config, '视频生成', getModel('官方h3-2k')).mode
    ).toBe('reference')
  })
  test('Sora keeps pixel dimensions without affecting other OpenAI-channel models', () => {
    const sora = { ...models[0], model: 'sora-2-pro' }
    expect(
      requestSize(sora, { ...config, resolution: '1080p', aspectRatio: '9:16' })
    ).toBe('1024x1792')
    expect(requestSize(models[0], { ...config, resolution: '1080p' })).toBe(
      '1080p'
    )
  })
})
