# 交期智能体 · 详细设计（插单影响面预演 + 交期解释器）

> 状态：**mist-docs 侧已实施**（确定性预演 P0、两段式提议 P1、规则版解释器 P3）；AI Tool / 自然语言入口（P2）与 LLM 叙述待 mist-team-server 实施 · 设计日期：2026-09-27 · 实施更新：2026-09-28
> 实施与本文的差异见 §13；最终决策见 §11 D14–D19。
> 上游依据：`research/01~04`（480 条来源调研）· `04-产品架构视角.md`（阶段一已上线）
> 范围：**阶段二 AI 提议层**——纯设计，不含语音报工/概率化交期（阶段三）

---

## 0. 一句话

在已上线的交期看板之上，加一层 **"只提议、不执行"** 的 AI：插单前算清**会顶掉谁**，交期存疑时说清**为什么**，让人确认后才落库。

---

## 1. 目标与非目标

### 1.1 目标

| # | 目标 | 验收口径 |
|---|------|---------|
| G1 | **插单影响面预演**：插入/调整一单前，列出被顶掉的订单、各自新交期、风险标记 | 计划员 10 秒内看到影响面，而非自己脑补 |
| G2 | **交期解释器**：回答"这单为什么可能晚" | 基于 `md_deadline_events` 留痕 + 负荷数据作答，标注证据来源 |
| G3 | **提议需人确认**：任何落库操作都走 ProposedAction 两段式 | 无一次静默写入；每次确认/驳回都留痕 |
| G4 | 全程可审计可回放 | 每次预演有 `run_id`，trace 可在 agent.html 回放 |

### 1.2 非目标（明确不做）

- ❌ **全能 APS / 全局最优排程**——调研结论：小厂最优排程第二天就废
- ❌ 设备采集、BOM 级 MRP、视觉质检——不碰硬件/OT
- ❌ AI 直接改表——违反 `internal/ai/agent.go` 的核心设计（*"can propose, never execute"*）
- ❌ 语音报工、概率化交期——阶段三，依赖历史数据积累

---

## 2. 现状盘点（已核对代码）

### 2.1 阶段一已有能力（mist-docs）

| 能力 | 位置 |
|---|---|
| 台账 CRUD + 看板查询（status/risk/owner/due 区间筛选） | `internal/handler/deadline.go` |
| 分级提醒调度器（30 分钟一扫，`uk_once` 幂等） | `internal/scheduler/reminder.go` |
| 变更留痕（`reason` 字段已预留为 AI 燃料） | `md_deadline_events` |
| 站内通知 | `createInAppNotification` → `md_notifications` |
| Webhook 出站（含企微机器人可复用） | `internal/handler/webhook.go` |
| 提醒规则 CRUD（T-N、channel、target 可配） | `deadline.go:532+` |

**表结构关键点**（`internal/database/mysql.go`）：
- `md_deadlines`：`due_date` + `team_id` 组合索引、`status` 独立列（非推导值）、`priority` 含 `inserted`
- `md_reminder_log`：`UNIQUE(rule_id, deadline_id)` 幂等
- `md_deadline_events`：`event_type/field/old_value/new_value/reason/actor_id`

### 2.2 AI 骨架已有能力（mist-team-server）

| 能力 | 位置 |
|---|---|
| `ProposedAction`（inert，只提议不执行） | `internal/ai/agent.go:59` |
| `Tool` 接口 + `ToolRegistry`（`ReadOnly()` 可标记只读工具，可 Subset/Filter 按角色裁剪） | `internal/ai/tools.go:41` |
| 现成工具：`list_hosts` / `search_snippets` / `search_command_logs` / `analyze_command` / `propose_command` / `tool_output_read` | `tools.go:164~602` |
| 分级授权 + TTL（`safe/readonly/all`） | `internal/ai/grants.go` |
| 全链路 trace（run_start/step/tool_call/…/run_end）+ 回放 | `events.go` `trace_repo.go` `web/agent.html` |
| 敏感脱敏 | `redact.go` |
| AI 运行时路由 | `routes_ops.go:113` `POST /teams/:team_id/ai/agent`（Viewer+ 可用） |

### 2.3 关键缺口

1. mist-docs **无 AI 能力、无调度外的计算层**；mist-team-server **无台账读写能力**
2. 两服务**同库 `mist_team`**，但 mist-team-server **没有跨服务调用 mist-docs HTTP 的现成 client**（只有 `DefaultDocsBaseURL` 常量用于链接拼接）
3. 没有**排产/负荷模型**（影响面计算的核心）
4. mist-docs 各路由仅 `JWTAuth + TeamAuth`（**未细分角色**，见 §8 风险 R3）

---

## 3. 总体架构

### 3.1 决策：AI 能力留在 mist-team-server，数据读写走同库直连

**方案对比**：

| 方案 | 说明 | 判定 |
|---|---|---|
| A. 在 mist-docs 内接 LLM | 重复建设 AI client/prompt/trace | ❌ |
| B. mist-docs HTTP 调 mist-team-server `/ai/agent` | 需新增服务间认证、网络失败处理 | ❌ 过早分布式化 |
| **C. mist-team-server 新增台账 Tool，同库直读** | 复用全套 AI 骨架；同库读是**已有先例**（`repository/fragment.go:181` JOIN `md_documents`） | ✅ |

```
┌─ MistTerm 客户端 / mist-docs 前端 ────────────────┐
│  插单预演对话框 · 交期解释卡片 · 确认/驳回按钮      │
└──────────────┬───────────────────────────────────┘
               │ POST /v1/teams/:id/ai/agent  (已有路由)
┌──────────────▼─────────────── mist-team-server ───┐
│ AgentLoop + Gate(L0/L1/L2) + ProposedAction        │
│ 新增 4 个台账 Tool（只读×3 + 提议×1）              │
│  · query_deadlines   · compute_load                │
│  · read_deadline_events · propose_schedule_change  │
│ trace/grants/redact 全复用                          │
└──────────────┬───────────────────────────────────┘
               │ 同库直连（mist_team MySQL）
┌──────────────▼───────────┐  ┌─────────────────────┐
│ md_deadlines 等 4 张台账表 │  │ md_deadline_events    │
│ (mist-docs 写，AI 读+提议) │  │ (留痕，解释器燃料)     │
└──────────────────────────┘  └─────────────────────┘
               │ 用户确认后（两段式）
┌──────────────▼───────────┐
│ mist-docs API / 直连写入   │ → 复用 TeamUpdateDeadline 的
│                           │   事件留痕与提醒联动
└──────────────────────────┘
```

**写路径铁律**：AI 永远只产出 `ProposedAction{action_type:"schedule_change", needs_confirm:true}`；确认后的实际写入由**一个专用 handler** 完成，该 handler 复刻 `TeamUpdateDeadline` 的留痕逻辑（`deadline.go:281` 注释明确每个字段变更都写 event）。**绝不让 Tool 直接 UPDATE。**

### 3.2 新增 Tool 清单

| Tool | 只读 | 输入 | 输出 |
|---|---|---|---|
| `query_deadlines` | ✅ | `{team_id, status?, due_from?, due_to?, owner_id?, q?, limit?}` | 台账行摘要（含 `days_left/risk`） |
| `compute_load` | ✅ | `{team_id, from, to, group_by: day\|owner}` | 区间内订单数/负荷、到期分布、逾期数 |
| `read_deadline_events` | ✅ | `{deadline_id, limit?}` | 留痕事件流（含 reason） |
| `propose_schedule_change` | ❌ | 提议体（见 §5.3） | `ToolResult{Action: ProposedAction}`，**不写库** |

全部通过 `ToolRegistry.Subset(...)` 按角色装配：Viewer 只读工具 + 提议工具；确认执行走 mist-docs 写接口的权限校验（第 6 节）。

**Prompt 约束**（写入 system prompt 前缀，保持可缓存）：
- 影响面结论**必须**引用工具返回的原始日期，禁止心算外推
- 证据不足时明说"数据不足"，禁止编造订单
- 结果中标注 `order_no` 作为可核对标识

---

## 4. 排产/影响面模型（轻量，边界严格）

### 4.1 模型范围

**单资源顺序模型**：把团队/产线简化为**有限个并行槽位（capacity）**，订单按优先级顺序占用时间窗。**不做**工序级、机台级、BOM 级——那些是 APS，明确不碰。

### 4.2 数据输入（全部已有或一次查询可得）

```sql
-- 产能配置：新增一张小表（见 §7 迁移1）
md_team_capacity(team_id, per_day INT, effective_from DATE)
-- 缺省 per_day = 1（单队列），配置后支持"每天 N 单"

-- 待排订单：状态非 done、未删除
SELECT id, order_no, title, due_date, start_date, priority, status, progress, owner_id
FROM md_deadlines
WHERE team_id=? AND status <> 'done' AND deleted_at IS NULL
ORDER BY FIELD(priority,'urgent','inserted','normal'), due_date ASC
```

### 4.3 算法（顺序滑窗 + 顺延传播）

```
输入: 插入单 new（或被改单），现有开放订单集合 O，capacity C（单/日）
1. 将 O ∪ {new} 按排序键（priority → due_date → 创建序）排序
2. 逐日填充：day = max(today, min_start)；当日已排数 < C 则排入，否则顺延到次日
3. 记录每单的 planned_finish
4. 违约判定: planned_finish > due_date  → 新增违约集
5. 增量对比: 与"插入前基线 planned_finish"逐单比较 → 新增延误集 Δ
   Δ = { order | new_finish > old_finish }  且每单记录 delay_days
6. 风险标注: Δ 中 order 含违约金条款(remark 关键词) / customer=重点客户 → flag
```

- **复杂度**：O(n log n)，n = 开放订单数（中小厂典型 < 500），毫秒级
- **基线缓存**：插入前的 baseline 由工具即时计算（同一算法跑一遍"无插入"分支），**不存库**——避免脏数据
- **单调性约束**：`new` 插入只会让部分单延后，不会提前；若计算出"大家都提前了"说明算法有 bug，测试要 pin 住

### 4.4 明确的行为约定

- `progress > 0` 的订单视为"已开工"，**不参与重排**（只读占位，结束时间不变）
- `status = 'overdue'` 的订单保持原位参与顺延（它们还在等交付）
- 插入单的 `priority = 'inserted'` 排序位在 urgent 之后、normal 之前——与现有 `validPriorities` 一致
- 结果中**同时给出**：新违约集（原无违 now 有）、延误集（延迟但未违约）、无影响 → 三档结论

---

## 5. 交互设计（两段式提议）

### 5.1 插单影响面预演

**入口**：
1. mist-docs 看板「插单预演」按钮 → 表单（订单/新交期/优先级）
2. 自然语言入口（MistTerm 对话框或看板输入框）：「把 A123 插到明天」

**流程**：
```
用户输入 ──► POST /teams/:id/ai/agent  (prompt + context)
              │
              ▼
        Agent 调 query_deadlines + compute_load
              │
              ▼
        确定性影响面计算（§4.3，工具内完成，不交给 LLM 算）
              │
              ▼
        LLM 组织自然语言结论 + ProposedAction{schedule_change}
              │
              ▼
   ┌──────────────────────────────────────┐
   │ 📋 预演卡片                           │
   │ 插入 A123（09-29，加急）               │
   │ 影响 3 单：                            │
   │   B456 → 延 2 天（09-28 → 09-30）⚠️   │
   │   C789 → 延 1 天（09-29 → 09-30）     │
   │   D012 → 延 3 天 ⚠️ 违约金条款         │
   │ 结论：2 单新违约，1 单延误              │
   │ [确认插入]  [修改方案]  [取消]          │
   └──────────────────────────────────────┘
        │ 用户点击 [确认插入]
        ▼
   POST /teams/:id/deadlines/:id/apply-proposal
   （权限校验 → 写台账 + 逐字段写 md_deadline_events(reason=插单影响)
     → 写 md_proposal_log → 触发受影响单的站内通知/webhook）
```

**关键 UI 约定**：
- 每个受影响订单都带**旧日期 → 新日期**的可视 diff，不允许只给结论
- `needs_confirm=true` 的提议**不提供**"一键全批"——逐提议确认
- 取消时也记录 `proposal_log.result='rejected'`（用于未来评估提议质量）

### 5.2 交期解释器

**入口**：看板订单行「为什么可能晚？」/ 老板问句
**证据链组装**（工具返回，LLM 只做叙述）：
1. `read_deadline_events`：交期改过几次、每次 reason、谁改的
2. `compute_load`：该单时间窗内的负荷与相邻单挤压
3. 逾期历史：`status='overdue'` 出现过几次
4. （可选）`search_snippets`：团队有没有"这类延期怎么处理"的片段

**输出格式（强制结构）**：
```json
{
  "conclusion": "可能延 2~3 天",
  "confidence": "medium",
  "evidence": [
    {"type": "event", "text": "09-20 交期从 09-28 改到 09-30，原因：铝材到货延迟", "ref": "evt_id"},
    {"type": "load",  "text": "09-27~09-30 该队列已排 5 单，超日产能 1 单/天", "ref": "compute_load"},
    {"type": "history","text": "近 30 天本单发生过 1 次逾期", "ref": "..."}
  ],
  "missing": ["未记录热处理工序外协周期"],
  "suggestion": "如需保交期，可将 C789 顺延 1 天（见预演）"
}
```
`missing` 字段是刻意的——**诚实暴露数据缺口**，同时反向推动用户补 `reason` 留痕。

---

## 6. API 设计

### 6.1 新增（mist-docs，确认执行侧）

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/teams/:team_id/deadlines/:id/apply-proposal` | editor+ | 应用已确认的变更；body: `{proposal_id, changes[], reason}` |
| POST | `/teams/:team_id/deadlines/preview-insert` | viewer+ | 纯计算预演（无 AI 也可用；确定性路径） |
| GET | `/teams/:team_id/proposals` | viewer+ | 提议历史（含 rejected） |

> **实际实现（2026-09-28）**：确认接口是 `POST /teams/:team_id/proposals/:id/apply`（另有 `POST /proposals/:id/reject`），body 只需 `{reason}`。服务端执行的是预演时**存进 `md_proposals` 的变更**，不接受客户端传来的 `changes[]`；“二次校验”改为比对基线指纹（开放订单的日期/优先级/进度等），在事务内锁定开放订单后比对，不一致则 409 并把提议标为 `stale`。另增 `GET` / `PUT /deadlines/capacity`（PUT 仅管理员）和 `GET /deadlines/:id/explain`（规则版解释器）。下面的请求示例保留为原设计。

`apply-proposal` 请求示例：
```json
{
  "proposal_id": "prp_xxx",
  "changes": [
    {"deadline_id": "dl_b456", "field": "due_date", "old": "2026-09-28", "new": "2026-09-30"},
    {"deadline_id": "dl_new",  "field": "__create__", "payload": {...}}
  ],
  "reason": "插单 A123 影响面确认"
}
```
**服务端必须二次校验**：`old` 与当前库值一致，否则 409（防基于过期预演的盲写）。

### 6.2 复用（mist-team-server）

- `POST /v1/teams/:team_id/ai/agent` — 入口不变，靠新 Tool 扩展能力
- `GET /v1/teams/:team_id/ai/agent/runs` / `agent/replay` — 回放

### 6.3 新表（原设计：mist-team-server 侧；实际：mist-docs 建表，见 D18/D19）

```sql
CREATE TABLE md_proposals (
  id VARCHAR(36) PRIMARY KEY,
  team_id VARCHAR(64) NOT NULL,
  run_id VARCHAR(64) DEFAULT '',
  user_id VARCHAR(64) NOT NULL,
  kind VARCHAR(32) NOT NULL COMMENT 'insert|date_change|explain',
  payload JSON NOT NULL,           -- 影响面结构（§5.1 卡片数据）
  baseline JSON NOT NULL,          -- 计算时的基线快照，apply 时用于 diff 校验
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending|applied|rejected|stale',
  applied_by VARCHAR(64) DEFAULT '',
  applied_at DATETIME NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_team_time (team_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

## 7. 数据变更与迁移清单

| # | 变更 | 库/服务 | 类型 |
|---|---|---|---|
| 1 | `md_team_capacity` 建表（缺省 1 单/日） | mist_team | 新表 |
| 2 | `md_proposals` 建表 | mist_team | 新表 |
| 3 | `md_deadline_events.event_type` 增加 `proposal_applied` | 迁移扩展 | 枚举扩展（VARCHAR，无需改表） |
| 4 | 新 Tool ×4 注册 | mist-team-server | 代码 |
| 5 | `preview-insert` / `apply-proposal` / `proposals` 路由 | mist-docs | 代码 |
| 6 | 看板预演卡片 + 解释器侧栏组件 | mist-docs/web | 代码 |

**无破坏性变更**：不动现有 4 张表结构，不改现有提醒语义。

> 实施状态：#1、#2、#3、#5、#6 已在 mist-docs 完成（两张表由程序启动时的 `database.Migrate` 幂等创建，也已写进 `docker/init-db.sql`）；#4（Tool 注册）属于 mist-team-server，未做。另外交期 `PUT` 现在也记录 `priority` 与 `start_date` 变更事件，解释器依赖这些记录。

---

## 8. 安全与权限

| 风险 | 对策 |
|---|---|
| R1 AI 越权写 | Tool 层无写权限；唯一写入口 `apply-proposal` 走 mist-docs `JWTAuth + TeamAuth` + editor 角色校验 |
| R2 过期基线盲写 | `apply-proposal` 强制 `old == current` 否则 409，前端提示"数据已变化，请重新预演" |
| R3 **mist-docs 路由未细分角色** | 现状 `teams.Use(TeamAuth())` 不区分 viewer/editor；`apply-proposal` **必须在 handler 内显式校验角色**（见 `permission.go` 的 `RequireTeamPermission` 模式），不能依赖中间件 |
| R4 提议被用于绕过审批 | `md_proposal_log` 全记录（含 rejected）；与审计事件同保留期 |
| R5 敏感信息（客户名/违约金）进 LLM | 仅经 `redact.go` 后进入 prompt；`payload` JSON 落库前同样过脱敏 |
| R6 计算结果被当成"承诺" | UI 文案固定为"预演/建议"，非"保证"；解释器 `confidence` 三档强制输出 |

**授权映射**（复用 grants 语义）：预演=readonly 级（Viewer 可发起）；apply=编辑权限（独立校验，不走 GrantLevel）。

---

## 9. 测试与验收

### 9.1 单元测试（确定性优先）

- **影响面算法**（核心，纯函数，table-driven）：
  - 插入导致 2 单顺延、1 单违约
  - `progress>0` 的单不参与重排
  - capacity=3 时 6 单占 2 天
  - 无影响插入 → 空 Δ（结果为"无影响"）
  - 单调性：插入后任何单的 finish 不得早于基线
  - 同优先级按 due_date 稳定排序
- `md_proposals` 状态机：pending→applied / pending→rejected / pending→stale
- `apply-proposal` 的 409 冲突路径
- 提醒联动：apply 后受影响单不重复触发已发过的提醒（`uk_once` 不破坏）

### 9.2 AI 侧测试

- 工具编入后 prompt 稳定（`Specs()` 顺序固定）
- 只读工具可被 `Subset` 正确裁剪
- LLM 无法通过 prompt 注入让 `propose_schedule_change` 直接写库（结构上不可能，测试 pin 住 ToolResult 无副作用）

### 9.3 端到端验收剧本

1. 造 5 单开放订单（含 1 单 progress=50、1 单 urgent）→ 触发插单预演 → 卡片列出正确影响集
2. 点确认 → 台账更新 + `md_deadline_events` 出现 `proposal_applied` + reason 完整
3. 用旧预演二次提交 → 409
4. 问"这单为什么可能晚" → 输出含 ≥2 条 evidence 且 ref 可点开
5. agent/replay 回放整条 run trace

---

## 10. 实施分期

| 期 | 内容 | 估时 | 依赖 |
|---|---|---|---|
| **P0** | `preview-insert` 确定性预演（无 AI）+ 前端卡片 + 单测 | 2~3 天 | 无 |
| **P1** | `apply-proposal` 两段式 + `md_proposals` + 留痕/通知联动 | 2 天 | P0 |
| **P2** | 4 个 Tool + 自然语言入口 + 提示词约束 + trace 接入 | 2~3 天 | P0/P1 |
| **P3** | 交期解释器（evidence 结构 + missing 字段）+ 回放验收 | 1~2 天 | P2 |

> 实施状态（2026-09-28）：P0 ✅、P1 ✅（mist-docs 侧）、P3 ✅ 规则版（无 LLM 叙述）、P2 ⬜（mist-team-server）。

**P0 独立有价值**：即使 LLM 接入延后，确定性预演已解决"插单前看不到影响面"这个最痛的空白——也符合"先解决一个真痛点"的调研建议。

---

## 11. 决策记录（ADR）

| # | 决策 | 理由 | 放弃的替代 |
|---|---|---|---|
| D1 | AI 工具在 mist-team-server，同库直读台账 | 复用全套 AI 骨架；同库 JOIN 已有先例 | mist-docs 内接 LLM（重复建设）；HTTP 跨服务（过早分布式化） |
| D2 | 影响面计算用**确定性算法**，LLM 只负责组织语言与意图解析 | 数字必须可信；LLM 算日期不可接受 | 让 LLM 直接推算（不可验证） |
| D3 | 写入必须经两段式 + 二次 diff 校验 | 满足"只提议不执行"铁律；防过期预演 | Tool 内直接落库 |
| D4 | 单资源顺序模型，capacity 可配 | 覆盖中小厂 80% 场景，O(n log n) 毫秒级 | 工序级排产（滑向 APS） |
| D5 | 提议历史含 rejected 全记录 | 未来评估提议质量、审计需要 | 只记成功 |

**实施决策（2026-09-28，编号沿用实施计划；D6–D13 是同一计划里认证、测试等其他决策）**

| # | 决策 | 实现位置 |
|---|---|---|
| D14 | capacity 按“订单数/天”，按生效日期分段（`md_team_capacity.effective_from` / `per_day`），未设置时 1 单/天 | `internal/schedule/preview.go`、`md_team_capacity` |
| D15 | 已开工的单（进度 > 0）保持原位置，不参与重排，结果里标 `started` | `schedule.Started` |
| D16 | 违约风险靠备注关键词（违约/赔偿/扣款）识别；另有团队级重点客户名单（`key_customers`，每行一个，不区分大小写），命中的受影响单打标 | `FlagPenalty` / `FlagKeyCustomer` |
| D17 | 确认只需一名编辑者（不做双人确认），记录 `decided_by` | `handler/proposals.go` |
| D18 | `payload` / `baseline` 用 LONGTEXT 存 JSON，不用 JSON 类型：MariaDB 与 MySQL 行为一致 | `database/mysql.go` |
| D19 | `md_proposals` 建在 mist-docs，与唯一写路径放在一起（原设计放 mist-team-server） | `database/mysql.go`、`handler/proposals.go` |

---

## 12. 开放问题（已定，见 D14–D17）

1. **capacity 语义**：按"订单数/天"还是按"加权工作量"？（建议 v1 订单数，字段留扩展位）
2. **apply 权限**：提议人 vs 受影响单 owner 是否需双人确认？（v1 单人 editor 即可，字段留 `applied_by`）
3. **违约金识别**：靠 remark 关键词（"违约/赔偿/扣款"）还是引入结构化字段？（v1 关键词，够用再结构化）
4. **自然语言入口放哪**：MistTerm 对话框 vs mist-docs 看板内输入框？（建议先看板内，MistTerm 侧随策略包叙事一起做）

结论：1 → D14（订单数/天）；2 → D17（单人 editor）；3 → D16（关键词 + 重点客户名单）；4 未定，随 P2 在 mist-team-server 侧决定（当前看板内只有表单式预演入口，没有自然语言输入）。

---

## 13. 实施与设计的差异（2026-09-28）

1. **确认路由**：`POST /proposals/:id/apply`（和 `/reject`），不是 `/deadlines/:id/apply-proposal`。
2. **服务端保存的变更**：每次预演都生成一条 pending 提议，存下“确认后要执行的变更”和基线指纹；确认时只执行存下的变更，客户端不能改。基线不一致 → 409 + `stale`；预演跨天也视为过期。
3. **解释器不调用 LLM**：`GET /deadlines/:id/explain` 返回规则拼出的结论（verdict / conclusion / confidence / evidence（带来源） / missing / assumptions / suggestion）和可带入插单预演的参数；没有变更记录时结论以“数据不足：”开头。文字目前只有中文。LLM 叙述留给 mist-team-server。
4. **表的位置**：`md_proposals`、`md_team_capacity` 在 mist-docs 建（D19），字段与 §6.3 略有不同：`decided_by` / `decided_at` 代替 `applied_by` / `applied_at`（驳回也记录），多了 `title` 和 `reason`，没有 `run_id`（等 AI 侧接入时再加）。
5. **确认后的联动**：写 `md_deadline_events`（`proposal_applied`），通知受影响单的负责人，投递 Webhook `deadline.proposal_applied`。
