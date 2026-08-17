# AI 上下文入口

这个目录是给 Codex、Claude、Cursor 等 AI 工具使用的轻量知识库。目标是先读少量稳定上下文，再按任务打开对应索引，减少重复 token 和无关记忆。

## 最小读取顺序

1. 先读仓库根目录的 `AGENTS.md`，确认项目硬性规则。
2. 再读本文件，选择和当前任务相关的索引。
3. 只打开必要的索引文件，再用 `rg` 定位具体代码。
4. 如果索引和代码冲突，以当前代码为准，并在完成后更新索引。

## 按任务选择索引

| 任务类型 | 优先读取 |
| --- | --- |
| 项目整体、目录定位、构建入口 | `project-map.md` |
| Go API、路由、控制器、数据库、服务层 | `backend-index.md` |
| 前端页面、组件、状态、样式、i18n | `frontend-index.md` |
| 渠道适配、计费、价格、认证、支付、部署 | `domain-index.md` |
| 历史长期决策、踩坑、人工约定 | `history-decisions.md` |
| 最新文件统计、路由列表、feature/channel 列表 | `generated/repo-snapshot.md` |

## 刷新生成索引

在仓库根目录运行：

```powershell
.\scripts\update-context-index.ps1
```

如需列出 `.chat-archive` 的归档元数据：

```powershell
.\scripts\update-context-index.ps1 -IncludeChatArchive
```

脚本只生成 `docs/ai-context/generated/repo-snapshot.md`。人工整理的知识请写入本目录其他 Markdown 文件。

## 维护原则

- 人工索引写长期有效的结构、规则、业务判断，不记录完整聊天内容。
- `generated/` 下的文件可以随时重建，不要手动维护关键知识。
- 新增大型功能后，优先更新对应的人工索引，再运行刷新脚本。
- 知识库正文中文为主，路径、命令、类型、包名保持英文。
