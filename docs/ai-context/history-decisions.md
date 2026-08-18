# 历史决策与稳定约定

这个文件只记录从历史对话、项目实践和人工决策中提炼出的长期有效信息。不要粘贴完整 `.chat-archive` 内容；需要追溯时只列归档元数据，再人工确认。

## 当前稳定决策

- AI 上下文知识库采用 Markdown + PowerShell 刷新脚本，不引入本地向量库或数据库。
- `.chat-archive` 不作为默认上下文输入，只提炼稳定决策、踩坑和长期约定。
- 当前项目副本不是 Git 工作树，索引刷新不能依赖 `git diff`。
- 知识库正文中文为主，路径、命令、类型、包名保持英文。
- 生成索引位于 `docs/ai-context/generated/`，可以重建；人工决策写在本文件或其他人工索引中。

## 从现有项目规则继承的长期约定

- 项目身份和组织身份相关信息受保护，不应删除、替换或弱化。
- 后端 JSON 编解码使用 `common/json.go` wrapper。
- 数据库代码必须同时兼容 SQLite、MySQL、PostgreSQL。
- 新增或修改 relay 请求 DTO 时要保留显式零值语义。
- 计费表达式任务必须先读 `pkg/billingexpr/expr.md`。
- 前端优先使用 `bun`，默认主题在 `web/default`，classic 主题只在明确需要时修改。
- Claude `/v1/messages` 遇到上游 `400 Invalid signature in thinking block` 时，在同一渠道内清理历史 signed thinking 后仅恢复重试一次；失效块只保存 SHA-256 指纹用于后续请求预清理，原始 thinking、签名和提示词不得写入日志。

## 待人工提炼

后续如果从 `.chat-archive` 发现稳定结论，请按这个格式补充：

```markdown
- 日期：YYYY-MM-DD
- 来源：`.chat-archive/<file>.md`
- 决策：一句话描述长期有效结论。
- 影响范围：涉及的模块、命令或文件。
```
