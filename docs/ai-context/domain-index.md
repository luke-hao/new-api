# 业务域索引

## 核心业务域

| 业务域 | 主要路径 | 注意点 |
| --- | --- | --- |
| 渠道管理 | `controller/channel*.go`、`model/channel*.go`、`service/channel*.go`、`web/default/src/features/channels/` | 后端 channel 字段、前端表单、渠道测试、批量操作常需要同步。 |
| Relay/provider | `router/relay-router.go`、`controller/relay.go`、`relay/channel/*`、`relay/common*` | 修改协议转换时要保护显式零值和 stream 行为。 |
| 模型与价格 | `controller/model*.go`、`controller/pricing.go`、`model/pricing*.go`、`setting/model*`、`web/default/src/features/pricing/`、`web/default/src/features/system-settings/models/` | 前后端模型倍率、分组倍率、价格展示容易耦合。 |
| 计费表达式 | `pkg/billingexpr/`、`service/text_quota.go`、`service/tiered_settle.go`、`service/billing*.go` | 修改前必须读 `pkg/billingexpr/expr.md`。 |
| 用户与认证 | `controller/user.go`、`middleware/auth.go`、`oauth/`、`controller/passkey.go`、`web/default/src/features/auth/` | OAuth、Passkey、JWT、注册开关和前端登录流程相关。 |
| Token/API key | `controller/token.go`、`model/token.go`、`service/token*`、`web/default/src/features/keys/` | 配额、分组、模型权限、用量统计常相关。 |
| 支付与订阅 | `controller/topup*.go`、`controller/subscription*.go`、`model/affiliate_rebate.go`、`service/*waffo*`、`service/billing_session.go`、`web/default/src/features/wallet/`、`web/default/src/features/subscriptions/` | Webhook、幂等、金额单位和可用支付方式要谨慎；邀请充值返利必须在支付成功事务内结算。 |
| 用量与日志 | `controller/usedata.go`、`controller/log.go`、`model/usedata*.go`、`model/log.go`、`web/default/src/features/usage-logs/` | 查询性能、时间范围、管理员权限和导出行为要核对。 |
| 系统设置 | `controller/option.go`、`model/option.go`、`setting/`、`web/default/src/features/system-settings/` | 一个配置项常横跨默认值、后端接口、前端表单、i18n。 |
| i18n | `i18n/`、`web/default/src/i18n/`、`docs/translation-glossary*.md` | 前端 key 用英文源文，后端用 go-i18n。 |
| 部署 | `Dockerfile*`、`docker-compose*.yml`、`new-api.service`、`rebuild-all.ps1`、`DEPLOY-KELE-AI.md` | 本地 Windows 构建优先看 `rebuild-all.ps1`。 |

## 项目硬规则

- 不删除或替换项目身份、组织身份、版权、README、包路径、Docker 镜像、品牌归属等受保护信息。
- Go 业务代码不要直接使用 `encoding/json` 进行 marshal/unmarshal，使用 `common/json.go`。
- 数据库逻辑必须兼容 SQLite、MySQL、PostgreSQL。
- 上游 relay 请求 DTO 的可选标量字段要保留显式零值，优先使用 pointer + `omitempty`。
- 计费表达式相关任务必须先读 `pkg/billingexpr/expr.md`。

## 修改联动提示

- 新增渠道：后端 channel type、adapter、模型获取/测试、前端渠道配置、文档可能都要同步。
- 新增系统配置：`setting` 默认值、option API、前端 settings section、i18n、持久化兼容都要检查。
- 新增前端页面：routes、feature、导航权限、接口封装、i18n、loading/error/empty 状态一起看。
- 新增支付或 webhook：金额单位、签名验证、幂等、失败响应、测试覆盖优先确认。
