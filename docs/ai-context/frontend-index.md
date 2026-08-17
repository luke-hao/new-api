# 前端索引

默认前端在 `web/default`，技术栈是 React 19、TypeScript、TanStack Router、TanStack Query、Base UI、Tailwind CSS、Rsbuild。优先使用 `bun` 运行脚本。

## 入口结构

| 路径 | 用途 |
| --- | --- |
| `web/default/src/routes/` | TanStack Router 文件路由。先从这里找页面入口。 |
| `web/default/src/features/` | 业务 feature 模块，页面主体通常在这里。 |
| `web/default/src/components/ui/` | 通用 UI 组件。 |
| `web/default/src/components/data-table/` | 表格组件和通用表格能力。 |
| `web/default/src/components/layout/` | 布局、导航、壳层。 |
| `web/default/src/stores/` | Zustand 全局状态，如 auth、system config、notification。 |
| `web/default/src/lib/` | 前端共享工具。 |
| `web/default/src/i18n/` | 前端 i18next 配置和语言 JSON。 |
| `web/default/src/styles/` | 全局 CSS、主题和预设。 |

## 路由到 Feature

常见流程：先在 `routes/**` 找 `createFileRoute`，再看它导入的 `features/<name>`。例如 channels 页面通常落到 `web/default/src/features/channels/`，系统设置落到 `web/default/src/features/system-settings/`。

高频 feature：

- `channels`：渠道列表、编辑抽屉、批量操作、模型映射、多 key、状态风险、上游更新。
- `system-settings`：站点、安全、认证、模型、计费、运营、内容、维护、集成配置。
- `models` / `pricing`：模型展示、价格查询、模型详情。
- `keys` / `usage-logs` / `wallet`：用户密钥、用量、余额和钱包。
- `auth` / `profile` / `users`：登录注册、用户资料、管理用户。
- `dashboard` / `rankings` / `performance-metrics`：统计和性能数据。

## 前端数据流

- API 封装通常放在 feature 内的 `api.ts`。
- 类型放 `types.ts`，局部工具放 `lib/`，复杂表单逻辑放 `hooks/`。
- 系统配置读取优先看 `web/default/src/stores/system-config-store.ts` 和 `features/system-settings/hooks/`。
- 表单和接口字段改动通常需要同步后端 DTO/controller/model。

## i18n 与样式

- 文案使用 `useTranslation()` 和 `t('English key')`。
- 翻译文件是 `web/default/src/i18n/locales/{lang}.json`，英文 key 扁平化。
- 修改文案后运行 `bun run i18n:sync`。
- 样式优先沿用现有组件和 Tailwind 约定，不引入新的设计体系。

## 验证命令

在 `web/default` 下：

```powershell
bun run typecheck
bun run build
bun run lint
```

只做小改动时，至少运行与改动相关的 typecheck 或 build。
