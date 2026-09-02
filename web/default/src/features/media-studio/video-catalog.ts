import type { VideoModelCapability } from './types'

export const VIDEO_GROUP_NAME = '视频生成'

type VideoCatalogDefinition = {
  id: string
  label: string
  note: string
  variants: string[]
}

type VideoCatalogSectionDefinition = {
  label: string
  items: VideoCatalogDefinition[]
}

export type AvailableVideoCatalogItem = VideoCatalogDefinition & {
  models: VideoModelCapability[]
}

export type AvailableVideoCatalogSection = {
  label: string
  items: AvailableVideoCatalogItem[]
}

const VIDEO_CATALOG_SECTIONS: VideoCatalogSectionDefinition[] = [
  {
    label: '官方区-稳定快速（推荐）',
    items: [
      {
        id: 'happyhorse',
        label: '【官方稳定版】快乐马（按秒）',
        note: '变体决定文生、首帧或多参考图模式，并固定 720p / 1080p；时长 4-15 秒。',
        variants: [
          'happyhorse-1.1-t2v-720p',
          'happyhorse-1.1-t2v-1080p',
          'happyhorse-1.1-i2v-720p',
          'happyhorse-1.1-i2v-1080p',
          'happyhorse-1.1-r2v-720p',
          'happyhorse-1.1-r2v-1080p',
        ],
      },
      {
        id: 'volcanoFutureH3',
        label: '【官方稳定版】MiniMax-H3',
        note: '必须添加参考图或参考音频，不支持文生和参考视频，分辨率由具体模型固定。',
        variants: ['官方h3-720p', '官方h3-1080p', '官方h3-2k'],
      },
      {
        id: 'officialWang30',
        label: '【官方稳定版】wang-3.0',
        note: '支持 4-30 秒，最多 10 图、5 视频和 5 音频。',
        variants: ['官方wang3.0-480p', '官方wang3.0-720p'],
      },
      {
        id: 'volcengineArk',
        label: '【官方不卡脸】sd2.5（可高并发）',
        note: '官方 2.5 支持 4-30 秒和 30 图/10 视频/10 音频，可高并发。',
        variants: ['【官方稳定版】2.5-480p', '【官方稳定版】2.5-720p'],
      },
      {
        id: 'volcengineSd20',
        label: '【官方不卡脸】sd2.0-720（可高并发）',
        note: '固定 720p；支持文生、首帧、首尾帧和多参考。',
        variants: [
          '【官方稳定版】sd2.0-720p-满血',
          '【官方稳定版】sd2.0-720p-fast',
        ],
      },
    ],
  },
  {
    label: '均衡区（相对稳定）',
    items: [
      {
        id: 'sd2',
        label: '【均衡性价比】sd2.5+2.0系列',
        note: 'SD2.0 变体支持 4-15 秒和 9 图/3 视频/3 音频；SD2.5 变体支持 4-30 秒和 30 图/10 视频/10 音频。',
        variants: [
          'sd2.0-720mini-不卡脸（按秒）',
          'sd2.0-720fast-不卡脸（按秒）',
          'sd2.0-720满血-不卡脸（按秒）',
          'sd2.0-1080mini-不卡脸（按秒）',
          'sd2.0-1080fast-不卡脸（按秒）',
          'sd2.0-1080满血-不卡脸（按秒）',
          'sd2.5-720均衡版',
          'sd2.5-480均衡版',
        ],
      },
      {
        id: 'stableValue86',
        label: '【比较稳定的】sd2.0+2.5',
        note: '固定 720p；2.0 支持 4-15 秒，2.5 支持 4-30 秒。',
        variants: [
          '【稳定】sd2.0-720满血（按秒）',
          '【稳定】sd2.0-720fast（按秒）',
          '【稳定】sd2.5-720p（按秒）',
        ],
      },
      {
        id: 'wang30',
        label: '【比较稳定的】wang-3.0',
        note: '支持 4-30 秒，最多 10 张图片，不支持参考视频和参考音频。',
        variants: ['wang-3.0-480p', 'wang-3.0-720p'],
      },
    ],
  },
  {
    label: '特惠区',
    items: [
      {
        id: 'sd25',
        label: 'sd-2.5特惠合集',
        note: '支持 4-30 秒，最多 30 图、10 视频、10 音频。',
        variants: [
          'sd-2.5-480p不卡脸(按秒)',
          'sd-2.5-720p不卡脸(按秒)',
          'sd-2.5-轮换渠道（按次）',
        ],
      },
      {
        id: 'sd900',
        label: 'sd-720特惠-900合集（0.8-2元）',
        note: '固定 720p、15 秒；仅支持 1-9 张多参考图。',
        variants: [
          'sd-720满血-900（不售后）',
          'sd2.0-720fast（特惠）',
          'sd2.0-720mini（特惠）',
        ],
      },
      {
        id: 'sd933',
        label: 'sd-720满血-933合集（2-5元）',
        note: '固定 720p；支持文生、首帧、首尾帧和多参考，时长 4-15 秒，最多 9 图、3 视频、3 音频。',
        variants: [
          'sd-720满血-933（按次）',
          'sd-720满血-933-备用（按次）',
          'sd-720满血-933（较慢）',
        ],
      },
      {
        id: 'sdPerUseAd',
        label: '【Adobe渠道】sd2.0',
        note: '具体模型固定比例和分辨率，固定 15 秒。',
        variants: [
          'sd2.0-480fast-ad渠道16x9',
          'sd2.0-480fast-ad渠道9x16',
          'sd2.0-480满血-ad渠道16x9',
          'sd2.0-480满血-ad渠道9x16',
          'sd2.0-720fast-ad渠道16x9',
          'sd2.0-720fast-ad渠道9x16',
          'sd2.0-720满血-ad渠道16x9',
          'sd2.0-720满血-ad渠道9x16',
          'sd2.0-1080满血-ad渠道16x9',
          'sd2.0-1080满血-ad渠道9x16',
        ],
      },
      {
        id: 'testChannel',
        label: '测试渠道（非管理勿用）',
        note: '按具体模型固定 480p、720p 或 1080p，支持 4-30 秒和多模态参考。',
        variants: [
          '测试模型480p-按次',
          '测试模型720p-按次',
          '测试模型1080p-按次',
          '测试模型480p-按秒',
          '测试模型720p-按秒',
          '测试模型1080p-按秒',
          '开源h3-480p（特惠版）',
          '开源h3-768p（特惠版）',
        ],
      },
    ],
  },
  {
    label: '扩展区',
    items: [
      {
        id: 'omni',
        label: 'omni',
        note: '生成模型支持最多 5 图；编辑模型支持最多 2 个参考视频；固定 720p、10 秒。',
        variants: [
          'omni-fast-视频生成（无水印）',
          'omni-fast-视频生成（带水印）',
          'omni-fast-视频编辑（无水印）',
          'omni-fast-视频编辑（带水印）',
        ],
      },
      {
        id: 'grok15preview',
        label: 'GROK1.5-Preview（可多参）',
        note: '支持文生、单首帧和最多 7 张多参考图。',
        variants: [
          'grok-imagine-video-1.5-preview',
          'grok-1.5-官转接口',
          'grok-1.5-备用接口',
          'grok-1.5-多参接口',
          'grok-video-1.5（按秒）',
        ],
      },
    ],
  },
]

export function buildAvailableVideoCatalog(
  models: VideoModelCapability[]
): AvailableVideoCatalogSection[] {
  const modelsByName = new Map(models.map((model) => [model.model, model]))

  return VIDEO_CATALOG_SECTIONS.map((section) => ({
    label: section.label,
    items: section.items
      .map((item) => ({
        ...item,
        models: item.variants
          .map((modelName) => modelsByName.get(modelName))
          .filter((model): model is VideoModelCapability => model !== undefined),
      }))
      .filter((item) => item.models.length > 0),
  })).filter((section) => section.items.length > 0)
}

export function flattenAvailableVideoCatalog(
  sections: AvailableVideoCatalogSection[]
): VideoModelCapability[] {
  return sections.flatMap((section) =>
    section.items.flatMap((item) => item.models)
  )
}

export function findAvailableVideoCatalogItem(
  sections: AvailableVideoCatalogSection[],
  modelName: string
): AvailableVideoCatalogItem | undefined {
  for (const section of sections) {
    const item = section.items.find((candidate) =>
      candidate.models.some((model) => model.model === modelName)
    )
    if (item) return item
  }
  return undefined
}

export function findAvailableVideoCatalogItemByID(
  sections: AvailableVideoCatalogSection[],
  id: string
): AvailableVideoCatalogItem | undefined {
  for (const section of sections) {
    const item = section.items.find((candidate) => candidate.id === id)
    if (item) return item
  }
  return undefined
}
