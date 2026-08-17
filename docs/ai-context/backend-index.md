# 后端索引

## 请求链路

常规链路是 `main.go` -> `router.SetRouter` -> middleware -> `controller/*` -> `service/*` -> `model/*`。Relay 请求会进入 `router/relay-router.go`，再走 `controller/relay.go` 和 `relay/*`。

## Router

| 文件 | 关注点 |
| --- | --- |
| `router/main.go` | 汇总 API、relay、dashboard、video、web 路由。 |
| `router/api-router.go` | 管理端和用户端 REST API：user、channel、token、option、pricing、subscription、models 等。 |
| `router/relay-router.go` | OpenAI/Gemini/Midjourney/Suno 等兼容 API 转发入口。 |
| `router/video-router.go` | video、Kling、Jimeng 等视频相关代理入口。 |
| `router/web-router.go` | 前端静态资源、SPA fallback、gzip/cache。 |
| `router/dashboard.go` | dashboard 兼容路由。 |

## Controller / Service / Model

- `controller/` 负责 Gin handler、请求参数、权限和响应格式。复杂业务不要长期堆在 controller。
- `service/` 放核心业务：渠道选择、计费、token 统计、任务轮询、支付、文件、排行榜、通知等。
- `model/` 放 GORM 模型、数据库访问、缓存、初始化和跨数据库迁移。
- 数据库代码必须同时兼容 SQLite、MySQL、PostgreSQL。原始 SQL 要留意引号、布尔值和 SQLite 限制。
- JSON marshal/unmarshal 必须使用 `common/json.go` 中的 wrapper。

## Relay

| 路径 | 关注点 |
| --- | --- |
| `controller/relay.go` | 统一 relay handler、错误处理、计费前后流程。 |
| `relay/relay_adaptor.go` | adapter 选择和通用 relay 调度。 |
| `relay/common/` | relay 请求上下文、计费信息、通用结构。 |
| `relay/common_handler/` | 兼容格式 handler。 |
| `relay/channel/*` | 各上游 provider adapter，如 `openai`、`claude`、`gemini`、`aws`。 |
| `relay/helper/` | 分组倍率、计费辅助、模型能力辅助。 |

新增或修改渠道时，先定位 `relay/channel/<provider>`，再确认 `constant/` 中 channel 类型、`model/channel.go`、前端渠道表单配置是否需要同步。若 provider 支持 `StreamOptions`，检查 `streamSupportedChannels`。

## 配置与 i18n

- `setting/` 管理系统级配置、模型倍率、运营开关、支付配置、性能设置。
- 后端 i18n 位于 `i18n/`，语言文件是 `i18n/locales/*.yaml`。
- 用户可配置项通常还会关联 `controller/option.go`、`model/option.go` 和前端 `system-settings`。

## 测试入口

- 后端单测：`go test ./...`
- 高风险区域已有测试示例：`service/*_test.go`、`controller/*_test.go`、`model/*_test.go`。
- 计费、支付、数据库兼容、relay 格式转换属于高风险区域，修改后优先补充或运行相关测试。
