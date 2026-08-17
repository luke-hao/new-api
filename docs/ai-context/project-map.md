# 项目地图

`new-api` 是 Go 后端 + React 前端的大模型网关与 AI 资产管理系统。后端负责统一 API、渠道转发、计费、用户与管理功能；前端默认主题位于 `web/default`。

## 顶层入口

| 路径 | 用途 |
| --- | --- |
| `main.go` | 服务启动、初始化、静态资源和 router 绑定入口。 |
| `router/` | HTTP 路由注册，按 API、relay、dashboard、video、web 拆分。 |
| `controller/` | Gin handler，处理请求绑定、权限判断、调用 service/model。 |
| `service/` | 业务逻辑、计费、渠道选择、文件、任务、支付、统计等。 |
| `model/` | GORM 模型、数据库访问、缓存、初始化和跨数据库迁移。 |
| `relay/` | 上游模型 API 转发、协议转换、provider adapter。 |
| `setting/` | 系统配置、倍率、模型、运营、支付、性能等配置加载。 |
| `common/` | 共享工具，包含 JSON wrapper、Redis、环境变量、加密、响应工具。 |
| `dto/` / `types/` / `constant/` | 请求响应结构、公共类型和常量。 |
| `web/default/` | 默认前端，React 19 + TypeScript + Rsbuild + Base UI + Tailwind。 |
| `web/classic/` | 旧版前端主题，通常只在同步/兼容任务中修改。 |
| `docs/` | 项目文档和本 AI 上下文知识库。 |

## 构建与运行

- 后端构建：`go build -o new-new-api.exe .`
- 默认前端：进入 `web/default` 后使用 `bun run dev`、`bun run build`、`bun run typecheck`。
- 一键构建：仓库根目录运行 `.\rebuild-all.ps1`。
- Classic 前端构建仅在需要时通过 `.\rebuild-all.ps1 -Classic`。

## 优先搜索策略

- 查接口：先看 `router/api-router.go` 或 `router/relay-router.go`，再跳到对应 `controller/*`。
- 查业务行为：先看 `service/*`，再看 `model/*` 或 `relay/*`。
- 查前端页面：先看 `web/default/src/routes/**`，再看 `web/default/src/features/**`。
- 查配置项：优先搜索 `setting/`、`model/option.go`、`controller/option.go`、`web/default/src/features/system-settings/`。
- 查历史约定：先读 `history-decisions.md`，不要直接塞入 `.chat-archive` 全文。

## 生成快照

`generated/repo-snapshot.md` 由 `scripts/update-context-index.ps1` 生成，适合快速确认当前目录统计、路由摘要、feature/channel 列表。它是辅助索引，不替代源码。
