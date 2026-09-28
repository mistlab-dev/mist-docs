# 第三期规划：产品打磨 + 跨服务扩展（2026-08-27）

## 范围

两个方向并行：

### A. 官网/Portal 产品打磨
### B. 跨服务功能扩展（审计可视化 + 权限细分 + 统一管理）

---

## A. 官网/Portal 产品打磨

### A1. 主站改版（mistlab.dev）

**现状**：纯静态 HTML，功能展示型，无品牌感

**目标**：专业级产品官网，统一 MistTerm + MistDocs 品牌形象

**改动**：
- 新首页 Hero 区：产品定位更清晰（"团队 SSH 协作平台"）
- 导航重构：首页 / 产品 / 文档 / 下载 / 登录
- 产品对比矩阵：MistTerm（客户端）vs MistDocs（文档）vs Portal（管理）
- 新增 Showcase/使用场景区
- 底部 CTA：注册 / 联系我们
- 响应式优化（当前移动端体验一般）

**涉及文件**：
- `mist-website/index.html`（重写）
- `mist-website/css/style.css`（重构）
- 新增 `mist-website/css/variables.css`（设计系统）
- 新增 `mist-website/js/nav.js`（导航交互）

### A2. MistDocs 文档站 UI 统一

**现状**：`docs.mistlab.dev` 是独立 Vite+React 应用，风格与主站不统一

**改动**：
- 主题色与主站对齐（深色 + 品牌色 #6366f1）
- 导航栏样式统一
- 增加"返回主站"入口
- 文档首页增加产品介绍卡片

**涉及文件**：
- `mist-docs/web/src/`（React 组件）

### A3. Portal 管理面板美化

**现状**：`mist-team-server/web/` 是管理后台，功能完整但 UI 粗糙

**改动**：
- 统一设计系统（深色主题、卡片布局）
- 仪表盘首页：团队概况、活跃用户、存储用量
- 导航优化：侧边栏分组（团队管理 / 审计 / 设置）

**涉及文件**：
- `mist-team-server/web/`（前端代码）

---

## B. 跨服务功能扩展

### B1. 审计报表可视化

**现状**：`audit-dashboard-mockup.html` 有完整设计稿，需要接入真实 API

**改动**：
- 将 mockup 转为 React 组件，集成到 Portal 管理面板
- 后端 API：`GET /teams/:id/audit/stats`（KPI + 趋势 + 分布）
- 后端 API：`GET /teams/:id/audit/logs`（分页日志查询）
- 前端：KPI 卡片 + 操作类型分布图 + 每日趋势图 + Top 用户 + 命令日志表
- 支持时间范围筛选（7天/30天/自定义）

**涉及文件**：
- 后端：`mist-team-server/src/api/audit.go`（新增统计 API）
- 前端：`mist-team-server/web/src/components/AuditDashboard.tsx`
- 前端：`mist-team-server/web/src/components/AuditLogTable.tsx`

### B2. 团队 Snippets 权限细分

**现状**：团队 snippets 只有全员可见/不可见，粒度太粗

**改动**：
- 权限模型：admin / editor / viewer（三级角色）
- 目录级权限：可按文件夹设置角色
- 个人 snippets：仅自己可见（已有）
- 团队 snippets：按角色控制读写

**数据库变更**：
- 新增 `fragment_permissions` 表
- 新增 `team_roles` 表

**API 变更**：
- `POST /teams/:id/fragments/:fid/permissions` — 设置权限
- `GET /teams/:id/fragments/:fid` — 增加权限检查

**涉及文件**：
- 后端：`mist-team-server/src/models/fragment.go`
- 后端：`mist-team-server/src/api/fragment.go`
- 前端：`mist-team-server/web/src/components/FragmentPermissions.tsx`

### B3. 统一管理控制台

**现状**：MistTerm（客户端）、MistDocs（文档）、Portal（管理）三套系统独立

**目标**：Portal 作为统一入口，管理所有服务

**改动**：
- Portal 仪表盘：汇总 MistTerm 团队数据 + MistDocs 文档统计
- 统一用户管理：一套账号体系（已有 JWT SSO 基础）
- 统一团队管理：在 Portal 管理团队成员、权限、存储
- 跨服务导航：Portal 内跳转 MistDocs 文档

**涉及文件**：
- 后端：`mist-team-server/src/api/dashboard.go`（汇总统计）
- 前端：`mist-team-server/web/src/pages/Dashboard.tsx`
- 前端：`mist-team-server/web/src/components/TeamOverview.tsx`

---

## 执行优先级

| 阶段 | 任务 | 预估工时 | 依赖 |
|------|------|----------|------|
| P0 | A1 主站改版 | 2-3天 | 无 |
| P0 | B1 审计报表可视化 | 3-4天 | 需后端 API |
| P1 | A2 MistDocs UI 统一 | 1-2天 | 无 |
| P1 | A3 Portal 美化 | 2-3天 | A1 设计系统 |
| P1 | B2 Snippets 权限细分 | 3-4天 | 数据库设计 |
| P2 | B3 统一管理控制台 | 4-5天 | B1 + B2 |

---

## 技术选型

### 前端
- **Portal/管理面板**：保持现有 React + TypeScript（mist-team-server/web/）
- **MistDocs**：保持 Vite + React + TypeScript
- **主站**：保持纯 HTML + CSS（轻量、SEO 友好），引入 CSS 变量设计系统

### 设计系统
- 深色主题（与现有产品一致）
- 主色：#6366f1（Indigo）
- 辅色：#22c55e（Green）/ #ef4444（Red）/ #f59e0b（Yellow）
- 字体：-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto

### 后端
- Go（mist-team-server）
- 数据库：MySQL（已有）
- API 风格：RESTful（已有）

---

## 验收标准

> **2026-09-23 复核**：本文写于 2026-08-27，勾选状态早已过期。下表由实际代码核对得出。

- [x] 主站改版完成，响应式通过测试 —— commit `f77bc4c`（新首页 + 三产品矩阵 + indigo 设计系统）
- [x] 审计 Dashboard 接入真实数据 —— `web/audit-dashboard.html` 已调用 `/v1/teams/:id/audit/stats`，
      后端路由见 `internal/app/routes_team.go:91`，图表可交互
- [x] Snippets 权限三级角色生效 —— 模型层 `viewer/editor/admin` 齐备
      （`internal/model/user.go`、`internal/model/fragment.go`），表 `fragment_permissions` 已建
- [x] Portal 统一管理面板可访问 —— 跨服务跳转已接入（admin 面板内链到 `docs.mistlab.dev`）
- [ ] 所有 API 有 go test 覆盖 —— MistDocs 大部分已覆盖，未到“全部”：2026-09 起 `go test ./tests/`
      在 CI 的 MySQL 8 上跑 150+ 个集成测试（不再静默跳过），`internal/` 各包有单元测试
      （排产/预演/解释器、鉴权与团队成员、限流、存储路径安全、加密、配置、webhook、ws）。
      个别接口（如 `/files/:filename`、`/openapi.json`、标签按文档查询）还没有专门测试，另行跟踪
- [x] 前端 npm run build 无报错
- [x] 生产部署 healthz 通过

### ⚠️ 文档需修正的技术前提

- **A2 写的「MistDocs 是 Vite + React」是错的** —— 实际是 **Vite + Vue 3 + Element Plus**，
  组件为 `.vue`（`mist-docs/web/src/`）。
- **A2 已实质完成**：品牌色 `#6366f1`、返回主站入口（`openPortal`）均在
  `mist-docs/web/src/layouts/MainLayout.vue` 与 `styles/dark.css` 中落地。
- **A3 / B3 部分完成**：Portal 主面板美化在进行中（`assets/css/admin.css` 已含品牌色），
  统一控制台的核心是跨服务跳转，已完成；汇总统计面板尚未完全对齐本文设想。

### 2026-09 新增（本文未覆盖）

- **交期看板**（`docs.mistlab.dev/deadlines`）：台账 + 分级提醒（T-7/T-3/T-1/当天/逾期）
  + 变更留痕 + 30 天准时率，已上线并端到端验证（scheduler 真实出提醒记录）。
  依据 `research/01-中小厂排产交期-需求整理.md` 的方向 A。
- **插单预演 → 两段式提议 → 交期解释器**（2026-09，phase-4/5/7）：`POST /deadlines/preview-insert`、
  `GET|PUT /deadlines/capacity`、`GET /proposals`、`POST /proposals/:id/apply|reject`、
  `GET /deadlines/:id/explain`。设计与最终决定见 `DESIGN-DEADLINE-AI.md`。
