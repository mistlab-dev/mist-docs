# mist-docs

企业文档协同服务 —— MistLab 生态的独立文档模块。

> 生产部署：`docs.mistlab.dev`（服务端 `85.137.247.166`，端口 8900，systemd 单元 `mist-docs`）

## 定位

- 独立部署、独立运行的文档服务
- **统一认证**：复用 Portal（`mist-team-server`）签发的 JWT，按 `team_id` 做团队级隔离
- 实时协同编辑（文档 + 表格）
- 与 MistTerm 团队片段双向联动（文档 ↔ 片段、段落级检索）
- 内网友好，无公网依赖

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go + Gin + MySQL |
| WebSocket | Go (gorilla/websocket) |
| 文档编辑器 | TipTap (ProseMirror) + Yjs |
| 表格编辑器 | Univer |
| 协同同步 | Yjs CRDT |
| 前端 | Vue 3 + Element Plus |
| 加密 | AES-256-GCM（`MISTDOCS_MASTER_FILE` 主密钥） |

## 项目结构

```
mist-docs/
├── cmd/server/main.go   # 入口 + 路由注册
├── internal/
│   ├── config/          # 配置
│   ├── database/        # 数据库连接与迁移
│   ├── model/           # 数据模型
│   ├── handler/         # HTTP Handler（team_api.go 为团队级 API 主体）
│   ├── middleware/      # SharedJWTAuth（Portal JWT）+ TeamAuth 成员校验 + 限流
│   ├── service/         # 业务逻辑
│   ├── ws/              # WebSocket Hub（协同中转）
│   ├── store/           # 文件存储
│   └── crypto/          # 信封加密
├── web/                 # 前端（Vue 3 + Vite）
├── docs/                # 设计文档
├── migrations/          # 数据库迁移
└── configs/             # 配置文件
```

## 快速开始

```bash
# 构建
go build -o mist-docs ./cmd/server

# 运行
./mist-docs -c configs/config.yaml
```

前端构建与部署（生产流程）：

```bash
cd web && npm run build
rsync -az --delete web/dist/ root@85.137.247.166:/var/www/mistdocs/web/
```

## API 概览

| 分组 | 路径前缀 | 鉴权 |
|------|----------|------|
| 探活 | `/healthz`、`/health` | 公开 |
| 登录 | `POST /api/auth/login` | 公开 |
| 当前用户 | `GET /api/auth/me`、`PUT /api/auth/password`、`POST /api/auth/logout` | JWT |
| 公开分享 | `GET /api/s/:token`、`/api/s/:token/info` | 公开 |
| 团队级 | `/api/teams/:team_id/**` | JWT + 团队成员 |
| 协同 | `WS /ws/teams/:team_id/docs/:doc_id` | JWT |

团队级 API 覆盖：文件夹树、文档 CRUD / 版本 / 锁定 / 分享 / 协作者 / 评论 / 导出、
回收站、标签、模板、权限、审计、收藏、存储、Webhook、媒体上传、通知，
以及与 MistTerm 联动的 `/documents/:id/fragments`、`/fragments-search`、`/docs/search`。

完整清单见 [`docs/UNIFIED-AUTH-DESIGN.md`](docs/UNIFIED-AUTH-DESIGN.md) 与运行时 `GET /api/openapi.json`。

## 与 MistTerm / Portal 的关系

- 不依赖 MistTerm 运行，但**共享认证与用户体系**：Portal 签发同一 secret 的 JWT，
  MistDocs 验证后查共享 `users` 表，并用 `team_members` 校验团队归属。
- 团队级请求命中 `/api/teams/:team_id/**` 时，中间件强制校验成员身份，
  非成员返回 `403 {"error":"不是该团队成员"}`。
- MistTerm 可将批量执行记录一键沉淀为 MistDocs 文档（服务端 `mist-team-server` 的
  `POST /v1/teams/:team_id/batch-exec/records/:id/export-doc`）。

## 相关文档

- [快速入门](docs/README.md)
- [技术设计](docs/DESIGN.md)
- [统一多租户认证设计](docs/UNIFIED-AUTH-DESIGN.md)
- [部署指南](docs/DEPLOYMENT.md)
- [WebSocket 协议](docs/WEBSOCKET.md)

