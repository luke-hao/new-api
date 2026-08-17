/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { createFileRoute } from '@tanstack/react-router'
import type { LucideIcon } from 'lucide-react'
import {
  BookOpen,
  ExternalLink,
  Image as ImageIcon,
  KeyRound,
  MonitorCog,
  TriangleAlert,
  WalletCards,
} from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { PublicLayout } from '@/components/layout'

type TutorialStep = {
  title: string
  description?: string
  image: string
  imageAlt: string
  additionalImages?: Array<{
    image: string
    imageAlt: string
  }>
  notice?: string
  link?: {
    href: string
    label: string
  }
}

type Tutorial = {
  id: string
  title: string
  tabTitle?: string
  description: string
  icon: LucideIcon
  steps: TutorialStep[]
}

const tutorials: Tutorial[] = [
  {
    id: 'getting-started',
    title: '新手教程',
    description: '创建 API 密钥，并将对应模型导入 CC Switch。',
    icon: KeyRound,
    steps: [
      {
        title: '进入控制台',
        description: '在网站首页点击顶部的“控制台”。',
        image: '/tutorials/getting-started/step-01.jpg',
        imageAlt: '网站首页进入控制台的位置',
      },
      {
        title: '打开 API 密钥',
        description: '在控制台左侧菜单中点击“API 密钥”。',
        image: '/tutorials/getting-started/step-02.jpg',
        imageAlt: '控制台 API 密钥入口',
      },
      {
        title: '创建密钥',
        description: '点击页面右上角的“创建 API 密钥”。',
        image: '/tutorials/getting-started/step-03.jpg',
        imageAlt: '创建 API 密钥按钮',
      },
      {
        title: '填写名称并选择分组',
        description:
          '名称可以随意填写，按自己的使用需求选择对应分组，然后保存修改。',
        notice: '分组一定要选，不能选择 default，也不能留空。',
        image: '/tutorials/getting-started/step-04.jpg',
        imageAlt: '创建密钥时选择对应分组并保存',
      },
      {
        title: '将密钥导出到 CC Switch',
        description:
          '回到 API 密钥列表，打开对应密钥右侧的更多菜单，点击“CC Switch”。',
        link: {
          href: 'https://ccswitch.io/zh',
          label: '下载 CC Switch',
        },
        image: '/tutorials/getting-started/step-05.jpg',
        imageAlt: 'API 密钥菜单中的 CC Switch 按钮',
      },
      {
        title: '选择对应应用',
        description: '根据自己的使用场景选择 Claude、Codex 或 Gemini。',
        image: '/tutorials/getting-started/step-06.jpg',
        imageAlt: 'CC Switch 应用类型选择',
      },
      {
        title: 'Claude 用户选择模型',
        description:
          '选择 Claude，并在各模型选项中选择以 claude 开头的对应模型。',
        image: '/tutorials/getting-started/step-07.jpg',
        imageAlt: 'CC Switch 的 Claude 模型配置',
      },
      {
        title: 'Codex 用户选择模型',
        description: '选择 Codex，主模型请选择以 gpt 开头的模型。',
        image: '/tutorials/getting-started/step-08.jpg',
        imageAlt: 'CC Switch 的 Codex 模型配置',
      },
      {
        title: 'Gemini 用户选择模型',
        description: '选择 Gemini，主模型请选择以 gemini 开头的模型。',
        image: '/tutorials/getting-started/step-09.jpg',
        imageAlt: 'CC Switch 的 Gemini 模型配置',
      },
    ],
  },
  {
    id: 'claude-client',
    title: 'Claude 客户端教程',
    tabTitle: 'Claude',
    description: '通过 CC Switch 配置并正常使用 Claude Desktop 客户端。',
    icon: MonitorCog,
    steps: [
      {
        title: '在 CC Switch 添加供应商',
        description:
          '打开 CC Switch，选择顶部的 Claude Desktop 图标，然后点击右上角的添加按钮。',
        image: '/tutorials/claude-client/step-01.jpg',
        imageAlt: '在 CC Switch 中选择 Claude Desktop 并添加供应商',
      },
      {
        title: '填写供应商、密钥和模型配置',
        description:
          '选择“自定义配置”，供应商名称可自行填写，官网链接和请求地址填写 https://code28.ccwu.cc，再从网站 API 密钥页复制对应密钥。',
        notice:
          '先删除原有的手动模型，再获取模型列表、添加模型并选择对应型号，最后保存。新人可暂时不开启 OAuth 认证。',
        image: '/tutorials/claude-client/step-02.jpg',
        imageAlt: 'Claude Desktop 自定义供应商名称和官网链接',
        additionalImages: [
          {
            image: '/tutorials/claude-client/step-03.jpg',
            imageAlt: '填写 Claude Desktop API 密钥和请求地址',
          },
          {
            image: '/tutorials/claude-client/step-04.jpg',
            imageAlt: '删除 Claude Desktop 原有手动模型',
          },
          {
            image: '/tutorials/claude-client/step-05.jpg',
            imageAlt: '获取并添加 Claude Desktop 模型后保存',
          },
        ],
      },
      {
        title: '开启 CC Switch 客户端设置',
        description:
          '回到 CC Switch 首页，点击左上角设置，在“通用”设置中按图开启对应选项。',
        notice:
          '开启“应用到 Claude Code 插件”“跳过 Claude Code 初次安装确认”和“关闭时最小化到托盘”。',
        image: '/tutorials/claude-client/step-06.jpg',
        imageAlt: '打开 CC Switch 设置',
        additionalImages: [
          {
            image: '/tutorials/claude-client/step-07.jpg',
            imageAlt: '开启 CC Switch 的三个 Claude 客户端选项',
          },
        ],
      },
      {
        title: '彻底退出并重新打开 Claude 客户端',
        description:
          '关闭客户端窗口后，检查任务栏右下角是否还有 Claude 图标；如有，请右键选择 Quit 彻底退出，再重新打开客户端。',
        image: '/tutorials/claude-client/step-08.jpg',
        imageAlt: '关闭窗口并从系统托盘彻底退出 Claude 客户端',
      },
      {
        title: '确认客户端已正常使用',
        description:
          '重新打开后能看到对话输入框和已配置模型，即表示客户端配置正常。',
        image: '/tutorials/claude-client/step-09.jpg',
        imageAlt: 'Claude Desktop 客户端正常使用界面',
      },
    ],
  },
  {
    id: 'image-generation',
    title: '生图教程',
    description: '在游乐场中生成图片或上传图片进行编辑。',
    icon: ImageIcon,
    steps: [
      {
        title: '进入图片游乐场',
        description: '点击左侧“游乐场”，再选择页面顶部的“图片”。',
        image: '/tutorials/image-generation/step-01.jpg',
        imageAlt: '游乐场的图片模式入口',
      },
      {
        title: '设置生图参数',
        description:
          '按图设置尺寸、质量、数量、分组和模型，填写提示词后开始生成。',
        notice: '建议数量保持为 1；只能选择当前分组内可用的生图模型。',
        image: '/tutorials/image-generation/step-02.jpg',
        imageAlt: '图片生成参数和模型选择说明',
      },
      {
        title: '上传图片进行编辑',
        description: '点击“编辑”切换模式，在下方上传原图。',
        notice: '编辑模式下的分组和模型要与上一步生图设置保持一致。',
        image: '/tutorials/image-generation/step-03.jpg',
        imageAlt: '图片编辑模式上传原图的位置',
      },
    ],
  },
  {
    id: 'recharge',
    title: '充值教程',
    description: '根据钱包当前显示的充值方式完成付款或兑换。',
    icon: WalletCards,
    steps: [
      {
        title: '方法一：直接在钱包充值',
        description: '当钱包显示“添加资金”时，选择金额或填写自定义金额后付款。',
        notice: '此界面表示当前没有限制收款，不需要手续费。',
        image: '/tutorials/recharge/step-01.jpg',
        imageAlt: '钱包页面直接添加资金',
      },
      {
        title: '方法二：前往充值页面',
        description:
          '当钱包提示充值功能受限时，点击左侧“充值”前往其他平台购买兑换码。',
        notice: '此方式需要收取手续费。',
        image: '/tutorials/recharge/step-02.jpg',
        imageAlt: '钱包受限时前往充值页面',
      },
      {
        title: '兑换充值码',
        description:
          '购买对应金额的兑换码后，回到充值页面，输入兑换码并点击“兑换额度”。',
        notice: '输入前请核对兑换码对应的充值金额，注意截图中的金额示例。',
        image: '/tutorials/recharge/step-03.jpg',
        imageAlt: '在充值页面输入兑换码并兑换额度',
      },
    ],
  },
]

function TutorialSteps({ tutorial }: { tutorial: Tutorial }) {
  return (
    <section aria-labelledby={`${tutorial.id}-heading`} className='pt-5'>
      <div className='border-b pb-5'>
        <h2 id={`${tutorial.id}-heading`} className='text-xl font-semibold'>
          {tutorial.title}
        </h2>
        <p className='text-muted-foreground mt-1 text-sm leading-6'>
          {tutorial.description}
        </p>
      </div>

      <ol className='divide-y'>
        {tutorial.steps.map((step, index) => (
          <li
            key={step.title}
            className='grid gap-4 py-7 md:grid-cols-[2.5rem_minmax(0,1fr)] md:gap-5'
          >
            <div
              aria-hidden='true'
              className='bg-foreground text-background flex size-8 items-center justify-center rounded-full text-sm font-semibold'
            >
              {index + 1}
            </div>
            <div className='min-w-0'>
              <h3 className='text-base font-semibold'>{step.title}</h3>
              {step.description ? (
                <p className='text-muted-foreground mt-1 max-w-3xl text-sm leading-6'>
                  {step.description}
                </p>
              ) : null}
              {step.notice ? (
                <div className='mt-3 flex max-w-3xl items-start gap-2 border-l-2 border-amber-500 bg-amber-50 px-3 py-2.5 text-sm leading-6 text-amber-950 dark:bg-amber-950/30 dark:text-amber-100'>
                  <TriangleAlert className='mt-1 size-4 shrink-0' />
                  <span>{step.notice}</span>
                </div>
              ) : null}
              {step.link ? (
                <a
                  href={step.link.href}
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary mt-3 inline-flex items-center gap-1.5 text-sm font-medium hover:underline'
                >
                  {step.link.label}
                  <ExternalLink className='size-3.5' />
                </a>
              ) : null}
              {[
                { image: step.image, imageAlt: step.imageAlt },
                ...(step.additionalImages ?? []),
              ].map((image) => (
                <img
                  key={image.image}
                  src={image.image}
                  alt={image.imageAlt}
                  loading='lazy'
                  decoding='async'
                  className='mt-4 h-auto w-full max-w-[720px] border bg-white object-contain'
                />
              ))}
            </div>
          </li>
        ))}
      </ol>
    </section>
  )
}

function TutorialDocs() {
  return (
    <PublicLayout>
      <div className='mx-auto w-full max-w-6xl py-4 md:py-8'>
        <header className='mb-6 flex items-start gap-3 border-b pb-6'>
          <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-md'>
            <BookOpen className='size-5' />
          </div>
          <div>
            <h1 className='text-2xl font-semibold'>使用教程</h1>
            <p className='text-muted-foreground mt-1 text-sm leading-6'>
              选择需要的教程，按步骤和截图完成操作。
            </p>
          </div>
        </header>

        <Tabs defaultValue={tutorials[0].id}>
          <TabsList className='grid h-auto w-full grid-cols-4 gap-1 p-1 md:w-fit'>
            {tutorials.map((tutorial) => {
              const Icon = tutorial.icon
              return (
                <TabsTrigger
                  key={tutorial.id}
                  value={tutorial.id}
                  className='min-h-10 min-w-0 px-2 md:px-4'
                >
                  <Icon className='size-4' />
                  <span className='truncate'>
                    {tutorial.tabTitle ?? tutorial.title}
                  </span>
                </TabsTrigger>
              )
            })}
          </TabsList>

          {tutorials.map((tutorial) => (
            <TabsContent key={tutorial.id} value={tutorial.id}>
              <TutorialSteps tutorial={tutorial} />
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </PublicLayout>
  )
}

export const Route = createFileRoute('/tutorial-docs')({
  component: TutorialDocs,
})
