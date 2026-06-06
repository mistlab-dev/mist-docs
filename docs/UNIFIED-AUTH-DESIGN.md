# MistLab 统一多租户架构设计

> 2026-06-06 · v2.0

## 1. 架构总览

```
                    mistlab.dev (Portal)
                    ├─ 官网（产品介绍、定价、下载）
                    ├─ 登录/注册/OAuth
                    ├─ 用户管理、团队管理
                    ├─ 订阅/计费
                    └─ 统一入口，登录后跳转子服务
                    
             ┌──────────────┴──────────────┐
             ▼                              ▼
    term.mistlab.dev              docs.mistlab.dev
    (MistTerm Team Server)        (MistDocs)
    ├─ 命令片段 CRUD               ├─ 文档/表格 CRUD
    ├─ 审计日志                    ├─ 文件夹树
    ├─ Vault SSH CA               ├─ 协同编辑
    ├─ API Keys / Webhooks        ├─ 分享/评论/通知
    └─ 命令审计策略                └─ 文档权限管理
```

**三服务，一数据库，共享认证。**

## 2. 域名与路由

| 域名 | 服务 | 端口 | 说明 |
|------|------|------|------|
| `mistlab.dev` | Portal | 80/443 | 官网 + 控制台 + 认证 |
| `term.mistlab.dev` | MistTerm | 8080 | 团队 API（Gin） |
| `docs.mistlab.dev` | MistDocs | 8900 | 文档 API（Gin） |
| `api.mistlab.dev` | MistTerm | 8080 | 兼容旧客户端（→ term） |

> `api.mistlab.dev` 目前指向 MistTerm，后续可考虑统一网关或保留。

## 3. 共享层

### 3.1 共享数据库表（同一 MySQL 实例/库）

```
users              — 全局用户表（email, username, display_name, is_admin）
teams              — 团队/租户（name, description）
team_members       — 成员关系（team_id, user_id, role: admin/editor/viewer）
subscriptions      — 订阅/套餐（user_id, plan, status）
email_verifications — 邮箱验证
oauth_accounts     — OAuth 绑定（Google/GitHub）
```

这些表由 Portal 服务管理（CRUD），其他服务只读。

### 3.2 共享 JWT

```go
// 统一 JWT claims（三个服务用同一个 secret 验证）
type Claims struct {
    UserID string `json:"uid"`
    jwt.RegisteredClaims
}
// Secret: 配置在各服务的 config.yaml 中，值相同
// Issuer: "mistlab"
```

- 登录：Portal 签发 token
- 验证：三个服务都能验证
- Token 存 localStorage（同根域 .mistlab.dev 下可共享）

### 3.3 跨域 SSO 方案

子域名间 localStorage 不共享（`mistlab.dev` ≠ `docs.mistlab.dev`）。

**方案：Cookie + 中央验证**

```
1. 用户在 mistlab.dev 登录 → Portal 写 httpOnly cookie: mist-token
2. 用户访问 docs.mistlab.dev → 前端发现没 token
3. 跳转 mistlab.dev/api/auth/check?redirect=docs.mistlab.dev
4. Portal 验证 cookie 有效 → 302 回 docs.mistlab.dev/?token=xxx
5. docs 前端拿到 token → 存 localStorage → 删 URL 参数
```

或者更简单：**所有子服务都用 mistlab.dev 域名下的路径**

```
mistlab.dev          → 官网
mistlab.dev/app      → Portal 控制台
mistlab.dev/term     → MistTerm API（或保持 api.mistlab.dev）
mistlab.dev/docs     → MistDocs
```

这样就不存在跨域问题，localStorage 直接共享。

**推荐先走子域名方案**（架构更干净），SSO 用 cookie 中转。

## 4. 各服务职责与数据隔离

### 4.1 Portal (mistlab.dev)

**服务：** mist-team-server（演进为 Portal）

**专属表：**
```
users, teams, team_members, subscriptions, oauth_accounts, email_verifications,
audit_events, command_audit_*, team_api_keys, team_webhooks,
vault_tenants, server_registrations, collaboration_sessions, terminal_recordings
```

**API 路由：**
```
POST /v1/auth/register       — 注册
POST /v1/auth/login          — 登录
POST /v1/auth/refresh        — 刷新 token
GET  /v1/auth/me             — 当前用户
GET  /v1/oauth/{provider}    — OAuth 登录

GET/POST /v1/teams            — 团队 CRUD
GET/POST /v1/teams/:id/members — 成员管理

GET/POST /v1/teams/:id/fragments — 片段（兼容旧客户端）
POST /v1/teams/:id/audit/batch   — 审计上报
...（现有 API 不变）
```

### 4.2 MistDocs (docs.mistlab.dev)

**服务：** mist-docs

**专属表：**
```
md_team_folders     — 团队文件夹树（新）
md_documents        — 文档（加 team_id）
md_versions         — 版本历史
md_permissions      — 文档/文件夹权限
md_shares           — 分享链接
md_comments         — 评论
md_notifications    — 通知
md_tags             — 标签
md_doc_tags         — 文档-标签关联
md_templates        — 模板
md_keys             — 加密密钥
md_webhooks         — Webhook
md_webhook_logs     — Webhook 日志
md_audits           — 审计日志
```

**废弃表（迁移后删除）：**
```
md_users          — 复用主站 users
md_departments    — 用 md_team_folders 替代
```

**API 路由：**
```
POST /api/auth/sso            — SSO token 交换（Portal token → Docs session）
GET  /api/auth/me             — 当前用户（查 users 表）

GET  /api/teams               — 我的团队列表（查 team_members + teams）
GET  /api/teams/:team_id/folders/tree    — 团队文件夹树
POST /api/teams/:team_id/folders         — 创建文件夹
PUT  /api/teams/:team_id/folders/:id     — 更新文件夹
DELETE /api/teams/:team_id/folders/:id   — 删除文件夹

GET  /api/teams/:team_id/documents       — 团队文档列表
POST /api/teams/:team_id/documents       — 创建文档
GET  /api/teams/:team_id/documents/:id   — 文档详情
PUT  /api/teams/:team_id/documents/:id   — 更新文档
DELETE /api/teams/:team_id/documents/:id — 删除文档

POST /api/teams/:team_id/documents/:id/share — 分享
...（权限、评论、通知等）
```

## 5. 认证流程

### 5.1 新用户首次访问 docs.mistlab.dev

```
1. GET docs.mistlab.dev → 前端 SPA 加载
2. 前端检查 localStorage('mist-docs-token') → 无
3. 跳转 https://mistlab.dev/login?redirect=https://docs.mistlab.dev
4. 用户在 Portal 登录 → Portal 写 cookie mist-token (domain=.mistlab.dev)
5. Portal 302 → https://docs.mistlab.dev/auth/callback?token=xxx
6. Docs 前端收 token → 存 localStorage → 调 /api/auth/me 确认用户
7. 进入文档页面
```

### 5.2 已登录用户再次访问

```
1. 前端检查 localStorage('mist-docs-token') → 有
2. 调 /api/auth/me → 验证 token → 返回用户信息
3. 直接进入
```

### 5.3 Token 过期

```
1. API 返回 401
2. 前端跳转 Portal 刷新 token
3. Portal 刷新后回调回来
```

## 6. 权限模型

### 6.1 团队角色（基础权限）

| 角色 | 团队管理 | 文档操作 | 文件夹管理 | 权限分配 |
|------|---------|---------|-----------|---------|
| admin | ✅ | 全部 | ✅ 创建/删/改 | ✅ |
| editor | ❌ | 创建/编辑/删自己的 | ❌ | ❌ |
| viewer | ❌ | 只读 | ❌ | ❌ |

### 6.2 文件夹 ACL（子权限）

团队 admin 可以在文件夹级别设置细粒度权限：

```
文件夹 "Q1 财报"
  ├─ 编辑者: 张三, 李四    （write）
  ├─ 查看者: 王五          （read）
  └─ 管理员: 赵六          （admin，可继续分配子权限）
```

权限继承：子文件夹继承父文件夹权限，除非显式覆盖。

### 6.3 文档级分享

保留现有 Google Docs 风格分享：
- 文档 owner 可直接邀请协作者（viewer/editor/admin）
- 不受文件夹 ACL 限制（显式分享优先级更高）

### 6.4 权限检查伪代码

```go
func CheckAccess(userID, teamID, resourceType, resourceID, required) bool {
    // 1. 验证团队成员关系
    role := GetTeamRole(userID, teamID)
    if role == "" { return false }
    
    // 2. team admin 全权限
    if role == "admin" { return true }
    
    // 3. 文档级显式分享
    perm := GetDirectPermission(userID, resourceType, resourceID)
    if perm != "" && perm >= required { return true }
    
    // 4. 文件夹 ACL（向上递归继承）
    folderID := GetParentFolder(resourceType, resourceID)
    for folderID != "" {
        perm := GetFolderACL(userID, folderID)
        if perm != "" && perm >= required { return true }
        folderID = GetParentFolder(folderID)
    }
    
    // 5. 团队角色兜底
    if required == "read" { return true }          // 成员都可读
    if required == "write" && role == "editor" { return true }
    
    return false
}
```

## 7. MistDocs 需要的改动

### 7.1 数据库 Schema 变更

```sql
-- 1. 新建团队文件夹表
CREATE TABLE md_team_folders (
  id          VARCHAR(36) PRIMARY KEY,
  team_id     VARCHAR(64) NOT NULL,
  parent_id   VARCHAR(36) DEFAULT '',
  name        VARCHAR(200) NOT NULL,
  sort_order  INT DEFAULT 0,
  created_by  VARCHAR(64) NOT NULL,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_team (team_id),
  INDEX idx_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 2. 文档表加 team_id
ALTER TABLE md_documents ADD COLUMN team_id VARCHAR(64) DEFAULT '' AFTER id;
ALTER TABLE md_documents ADD INDEX idx_team (team_id);

-- 3. 文件夹表加 team_id
ALTER TABLE md_folders ADD COLUMN team_id VARCHAR(64) DEFAULT '' AFTER id;
ALTER TABLE md_folders ADD INDEX idx_team (team_id);

-- 4. 审计表加 team_id
ALTER TABLE md_audits ADD COLUMN team_id VARCHAR(64) DEFAULT '';

-- 5. 权限表改语义（department → folder），不改结构
-- data migration: UPDATE md_permissions SET target_type='folder' WHERE target_type='department';

-- 6. 迁移完成后删除
-- DROP TABLE md_users;
-- DROP TABLE md_departments;
```

### 7.2 后端改动清单

| 文件 | 改动 |
|------|------|
| `config.go` | JWT secret 改为主站相同；加 `portal_url` 配置 |
| `middleware/auth.go` | 新 `SharedJWTAuth`，解析主站 JWT，查 users + team_members |
| `middleware/permission.go` | 三层权限检查（团队角色 → 文件夹 ACL → 文档分享） |
| `handler/user.go` | 删除 `Login`，保留 `Me`（查 users 表），加 `SSOCallback` |
| `handler/doc.go` | 所有操作加 team_id 过滤 |
| `handler/folder.go` | 新增，管理 md_team_folders |
| `handler/share.go` | 权限检查改用新模型 |
| `handler/collaborator.go` | target_type 从 department → folder |
| `model/model.go` | 新增 TeamFolder；User 改为读 users 表字段 |
| `database/mysql.go` | 新建 md_team_folders 表 migration |
| `cmd/server/main.go` | 路由加 /teams/:team_id 前缀；auth middleware 换新的 |

### 7.3 前端改动清单

| 文件 | 改动 |
|------|------|
| `stores/auth.ts` | token 来源改共享逻辑；login → 跳转 Portal |
| `utils/http.ts` | baseURL 改；token 来源统一 |
| `router/index.ts` | 未登录 → 跳 Portal login + redirect |
| `views/Login.vue` | 删除或改为跳转中转页 |
| `views/Docs.vue` | 加团队选择器 |
| `views/DocEditor.vue` | API 调用加 team_id |
| `views/admin/Departments.vue` | 改为 TeamFolders.vue（文件夹树管理） |
| `views/admin/Users.vue` | 删除（用户管理在 Portal） |
| `layouts/MainLayout.vue` | 侧栏调整：去掉用户/部门管理，加团队选择 |

### 7.4 Portal (mist-team-server) 改动

| 文件 | 改动 |
|------|------|
| `web/index.html` | 登录成功后支持 redirect 参数跳转 |
| `web/assets/js/auth.js` | 登录成功写 cookie (domain=.mistlab.dev) |
| `handler/auth.go` | login 成功写 cookie |
| `config.go` | JWT issuer 改为 `mistlab` |

## 8. 实施计划

### Phase 1: 数据库 Schema (1天)
- [ ] 创建 `md_team_folders`
- [ ] `md_documents` / `md_folders` / `md_audits` 加 `team_id`
- [ ] 清理 `md_departments` 重复数据
- [ ] 编写数据迁移脚本
- [ ] 验证 migration

### Phase 2: 认证统一 (1天)
- [ ] MistDocs JWT secret 改为 Portal 相同
- [ ] 新 `SharedJWTAuth` 中间件
- [ ] 删除 `Login` handler，加 `Me`（查 users 表）
- [ ] Portal 登录页支持 redirect 参数
- [ ] 跨域 token 传递方案实现
- [ ] 前端 auth store 适配

### Phase 3: API 改造 (2天)
- [ ] 所有 MistDocs API 加 `/teams/:team_id` 前缀
- [ ] 文档查询/创建/删除按 team_id 隔离
- [ ] 新文件夹 API（md_team_folders CRUD）
- [ ] 权限检查改用三层模型
- [ ] 协作者/分享适配

### Phase 4: 前端整合 (1.5天)
- [ ] 团队选择器组件
- [ ] Docs.vue / DocEditor.vue 适配
- [ ] 文件夹树管理页面
- [ ] 删除旧页面（部门管理、用户管理）
- [ ] 侧栏导航调整

### Phase 5: 部署与 Nginx (0.5天)
- [ ] Nginx 配置 term.mistlab.dev / docs.mistlab.dev
- [ ] SSL 证书（通配符 *.mistlab.dev 或单独）
- [ ] 编译部署三个服务
- [ ] 全流程 E2E 测试

### Phase 6: 清理 (0.5天)
- [ ] 删除 md_users、md_departments
- [ ] 清理废弃代码
- [ ] 更新文档

## 9. Nginx 配置草案

```nginx
# mistlab.dev — Portal + 官网
server {
    server_name mistlab.dev www.mistlab.dev;
    
    # 官网静态文件
    location / {
        root /var/www/mistlab;
        try_files $uri $uri/ /index.html;
    }
    
    # Portal API
    location /v1/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    
    # Portal 控制台
    location /app {
        proxy_pass http://127.0.0.1:8080;
    }
}

# term.mistlab.dev — MistTerm API（或保留 api.mistlab.dev）
server {
    server_name term.mistlab.dev api.mistlab.dev;
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# docs.mistlab.dev — MistDocs
server {
    server_name docs.mistlab.dev;
    
    # SPA 静态文件
    location / {
        root /var/www/mistdocs/web;
        try_files $uri $uri/ /index.html;
    }
    
    # Docs API
    location /api/ {
        proxy_pass http://127.0.0.1:8900;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    
    # WebSocket（协同编辑）
    location /ws/ {
        proxy_pass http://127.0.0.1:8900;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```
