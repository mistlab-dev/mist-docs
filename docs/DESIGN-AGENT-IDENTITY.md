# Agent 身份与最小权限 — 一页设计（#68）

> 状态：**方案，未实现**。本文回答 #65 Epic 复审（c-wind）要求的五个问题，
> 作为 T4（Runbook 可执行化）的前置条件。落地顺序：T6 方案 →（有需要时）
> T6 实现 → 才考虑 T4。

## 0. 现状与问题

MCP `/api/mcp` 与所有 AI 调用目前**借用户的 JWT**：agent 的每一步都以
「某人」的身份执行，审计日志里分不清是人点的还是 AI 跑的。这在只读召回
（#66）下风险可控，但一旦涉及写入（#67 草稿入库、未来 T4 执行命令），
「借身份」就意味着：权限粒度只有人的全部权限，撤销只能撤人，无法单独
吊销一个 agent。

## 1. 身份：独立 principal，绑定创建者

选 **独立 principal**（`agent:<id>`），不选用户子身份：

- 撤销粒度正确：吊销 agent 不影响人；人离职不清 agent
- 审计可区分：`md_audits.user_id` 直接写 agent id，一眼分清
- 表：新表 `md_agents`（id, team_id, name, creator_user_id, scopes,
  expires_at, revoked_at, created_at）——挂在团队下，不全局

不引入登录：agent 不设密码、不能走 Portal SSO，只以
`Authorization: Bearer <agent token>` 调 MCP（token = 随机 32 字节，
哈希入库，可轮换）。

## 2. 权限：按操作类型 + 文档范围白名单

`md_agents.scopes` 用 JSON 表达，两层限制：

```json
{
  "tools":  ["search_doc_content", "read_doc", "list_docs"],
  "docs":   ["doc-id-1", "doc-id-2"],   // 空数组 = 全部可 read 文档
  "expire": "2026-12-31T00:00:00Z"
}
```

- 工具层：白名单，未列出的工具直接拒绝（`create_doc` / `update_doc`
  默认不在任何 scope 里）
- 文档层：白名单为空 = 沿用 `HasTeamPermission` 的团队语义；非空 =
  只能碰列出的文档（在 `HasTeamPermission` 之前先查白名单）
- 不发明新权限位：最终判定仍走 `CheckTeamPermission`，agent 只是多了
  一道前置收窄

## 3. 审计：复用 md_audits，加一个字段

不建独立表。`md_audits` 加 `actor_type ENUM('user','agent') DEFAULT 'user'`
（`dropColumnIfExists` 模式迁移，见 `mysql.go` 惯例）：

- `user_id` = agent id，`user_name` = agent 名，`detail` 里带
  `{"agent_creator":"<user-id>","session":"<mistterm-session-id>"}`
- 现有审计筛选/导出/统计（`TeamListAudits` 等）零改动即可展示 agent 行为
- 每次工具调用必写审计，无例外——包括被拒绝的调用（action=`mcp.denied`）

## 4. 撤销与过期

- `expires_at` 默认 30 天，创建时可缩短不可延长
- `revoked_at` 非空即失效：MCP 入口中间件一次性检查，不逐工具判断
- 团队 admin 在权限页可列出/吊销本团队全部 agent（复用权限页 UI，
  不新做页面）
- token 轮换 = 旧行 `revoked_at` + 新行

## 5. MCP 工具映射

| 工具 | agent 默认 | 说明 |
|---|---|---|
| `search_docs` / `search_doc_content` / `list_docs` | ✅ 白名单内允许 | 只读，召回场景（#66）的主路径 |
| `read_doc` | ✅ 白名单内允许 | 文档范围白名单生效 |
| `create_doc` | ⚠️ 审批制 | 默认拒绝；开启后仍只产**草稿状态**文档，需人转正（对齐 #67「人确认后入库」） |
| `update_doc` | ❌ 默认禁止 | 自动改写已有文档在 T4 前一律不放行；即使未来开放，也只允许追加 blockquote 型「AI 附录」，不碰原文 |

REST API 不对 agent 开放——agent 只走 MCP 单一入口，攻击面最小。

## 6. 最小实现路径（估计，不含代码）

1. `md_agents` 表 + token 签发/校验中间件（挂在 MCP 路由前）
2. scopes 检查 + `md_audits.actor_type` 迁移
3. 权限页 agent 管理区块
4. MistTerm 侧用 agent token 替换用户 JWT 调 MCP（需 Tian 配合）

## 7. 明确不做的

- 不做 agent 之间的信任链 / 多级委托
- 不做全局 agent（跨团队）——一律团队内
- 不做 agent 直连数据库/文件系统，只走 MCP
