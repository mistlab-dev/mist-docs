# MistDocs 技术设计文档

## 1. 系统架构

```
用户浏览器
  │
  ├── 文件浏览器（部门树 + 文件列表）
  ├── 文档编辑器（TipTap + Yjs 协同）
  └── 表格编辑器（Univer + Yjs 协同）
  │
  ├── HTTP ──→ MistDocs 服务端（Go + Gin + MySQL）
  │              ├── 用户/部门管理
  │              ├── 文档 CRUD + 版本
  │              ├── 权限管控
  │              ├── 审计日志
  │              └── 文件存储
  │
  └── WebSocket ──→ MistDocs WS Hub（Go）
                      ├── 房间管理（按文档 ID）
                      ├── Yjs 更新广播
                      ├── 权限校验（加入房间时）
                      └── 持久化（定期存库）
```

## 2. 数据模型

### 2.1 用户与部门

#### departments 部门表（三级树形）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(36) | 主键 |
| name | varchar(100) | 部门名称 |
| parent_id | varchar(36) | 上级部门 ID（NULL=根部门） |
| sort_order | int | 排序 |
| status | tinyint | 1=启用 0=禁用 |
| created_at | datetime | |
| updated_at | datetime | |

约束：层级不超过三级（应用层校验）。

#### users 用户表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(36) | 主键 |
| username | varchar(50) | 登录名 |
| password | varchar(255) | bcrypt 哈希 |
| name | varchar(100) | 姓名 |
| email | varchar(100) | 邮箱（可选） |
| phone | varchar(20) | 手机（可选） |
| department_id | varchar(36) | 所属部门 |
| role | varchar(20) | super_admin / dept_admin / member |
| status | tinyint | 1=启用 0=禁用 |
| last_login_at | datetime | |
| created_at | datetime | |
| updated_at | datetime | |

### 2.2 文档

#### doc_folders 文件夹表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(36) | 主键 |
| name | varchar(200) | 文件夹名 |
| parent_id | varchar(36) | 上级文件夹 |
| department_id | varchar(36) | 所属部门 |
| created_by | varchar(36) | 创建人 |
| created_at | datetime | |
| updated_at | datetime | |

#### documents 文档表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(36) | 主键 |
| folder_id | varchar(36) | 所在文件夹 |
| department_id | varchar(36) | 所属部门（冗余） |
| title | varchar(200) | 文档标题 |
| type | varchar(20) | doc / sheet |
| file_path | varchar(500) | 磁盘文件路径 |
| file_size | bigint | 字节数 |
| version | int | 当前版本号 |
| status | tinyint | 1=正常 0=回收站 |
| created_by | varchar(36) | |
| updated_by | varchar(36) | |
| created_at | datetime | |
| updated_at | datetime | |

#### doc_versions 版本表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(36) | 主键 |
| document_id | varchar(36) | 文档 ID |
| version | int | 版本号 |
| file_path | varchar(500) | 该版本文件路径 |
| file_size | bigint | |
| created_by | varchar(36) | |
| created_at | datetime | |

### 2.3 权限

#### doc_permissions 权限表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(36) | 主键 |
| resource_type | varchar(10) | folder / document |
| resource_id | varchar(36) | 文件夹或文档 ID |
| target_type | varchar(10) | department / user |
| target_id | varchar(36) | 部门 ID 或用户 ID |
| permission | varchar(10) | read / write / admin |
| inherit | tinyint | 1=子项继承 |
| created_by | varchar(36) | |
| created_at | datetime | |

**权限计算规则**（优先级从高到低）：

1. 用户级权限（target_type=user）
2. 部门级权限（target_type=department）
3. 继承父文件夹权限（inherit=1 时向上查找）
4. 默认规则：本部门可读写，其他部门不可见

**权限级别**：

| 级别 | 查看 | 编辑 | 删除 | 管理权限 |
|------|------|------|------|---------|
| read | ✅ | ❌ | ❌ | ❌ |
| write | ✅ | ✅ | ✅ | ❌ |
| admin | ✅ | ✅ | ✅ | ✅ |

### 2.4 审计

#### doc_audits 审计日志表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(36) | 主键 |
| user_id | varchar(36) | 操作人 |
| user_name | varchar(100) | 操作人姓名（冗余） |
| department_id | varchar(36) | 操作人部门 |
| action | varchar(30) | 操作类型 |
| resource_type | varchar(10) | folder / document |
| resource_id | varchar(36) | 资源 ID |
| resource_name | varchar(200) | 资源名称（冗余） |
| detail | text | JSON 详情 |
| ip | varchar(50) | 客户端 IP |
| created_at | datetime | |

**操作类型**：

| action | 说明 | detail 示例 |
|--------|------|------------|
| login | 登录 | - |
| view | 查看文档 | - |
| create_doc | 创建文档 | `{"type":"doc"}` |
| edit_doc | 编辑保存 | `{"version":3}` |
| delete_doc | 删除文档 | - |
| restore_doc | 恢复文档 | - |
| create_folder | 创建文件夹 | - |
| delete_folder | 删除文件夹 | - |
| move | 移动 | `{"from":"/a","to":"/b"}` |
| download | 下载 | - |
| set_permission | 设置权限 | `{"target":"dept_x","perm":"read"}` |
| remove_permission | 移除权限 | - |
| import_user | 导入用户 | `{"count":50}` |
| export | 导出审计日志 | - |

## 3. API 设计

### 3.1 认证

```
POST   /api/auth/login                  # 登录
POST   /api/auth/logout                 # 登出
GET    /api/auth/me                     # 当前用户信息
PUT    /api/auth/password               # 修改密码
```

### 3.2 用户与部门管理（super_admin / dept_admin）

```
GET    /api/departments                  # 部门树
POST   /api/departments                  # 创建部门
PUT    /api/departments/:id              # 更新部门
DELETE /api/departments/:id              # 删除部门

GET    /api/users                        # 用户列表（支持分页/筛选）
POST   /api/users                        # 创建用户
PUT    /api/users/:id                    # 更新用户
DELETE /api/users/:id                    # 禁用用户
PUT    /api/users/:id/reset-password     # 重置密码
POST   /api/users/import                 # CSV 批量导入
POST   /api/departments/import           # CSV 批量导入部门
```

### 3.3 文档管理

```
GET    /api/docs/tree                    # 文件夹树（按权限过滤）
POST   /api/docs/folders                 # 创建文件夹
PUT    /api/docs/folders/:id             # 重命名文件夹
DELETE /api/docs/folders/:id             # 删除文件夹

GET    /api/docs/documents?folder_id=    # 文档列表
POST   /api/docs/documents               # 创建文档
PUT    /api/docs/documents/:id           # 更新文档（重命名/移动）
DELETE /api/docs/documents/:id           # 删除文档（移到回收站）

GET    /api/docs/documents/:id/content   # 获取文档内容
PUT    /api/docs/documents/:id/content   # 保存文档内容

GET    /api/docs/documents/:id/versions  # 版本列表
POST   /api/docs/documents/:id/restore   # 恢复版本
GET    /api/docs/trash                   # 回收站
POST   /api/docs/trash/restore/:id       # 恢复
DELETE /api/docs/trash/purge/:id         # 永久删除
```

### 3.4 权限管理

```
GET    /api/permissions?resource_id=     # 获取资源权限列表
POST   /api/permissions                  # 设置权限
DELETE /api/permissions/:id              # 删除权限
GET    /api/permissions/check            # 检查当前用户对某资源的权限
```

### 3.5 协同 WebSocket

```
WS     /ws/docs/:doc_id?token=xxx        # 加入文档编辑房间
```

**消息协议**（基于 Yjs sync protocol）：

```json
// 客户端 → 服务端
{"type": "sync-step-1", "data": [...]}
{"type": "sync-step-2", "data": [...]}
{"type": "update", "data": [...]}

// 服务端 → 客户端
{"type": "sync-step-1", "data": [...]}
{"type": "sync-step-2", "data": [...]}
{"type": "update", "data": [...]}
{"type": "awareness", "data": {"user": {"name":"张三","color":"#f00"}, "cursor": {"line":5,"col":10}}}
{"type": "join", "user": {"id":"xxx","name":"李四"}}
{"type": "leave", "user": {"id":"xxx"}}
```

### 3.6 审计

```
GET    /api/audits                       # 审计日志查询
  ?user_id=                             # 按用户
  &department_id=                        # 按部门
  &action=                              # 按操作类型
  &resource_id=                         # 按资源
  &start_date=&end_date=                # 时间范围
  &page=&page_size=                     # 分页

GET    /api/audits/export                # 导出 CSV
GET    /api/audits/stats                 # 统计（活跃用户、热门文档等）
```

## 4. 文件存储设计

### 4.1 目录结构

```
/var/lib/mist-docs/files/
├── {department_id}/                     # 按部门隔离
│   ├── {folder_id}/
│   │   ├── {doc_id}/
│   │   │   ├── v1.dat                   # 版本文件（Yjs 编码）
│   │   │   ├── v2.dat
│   │   │   └── current.dat              # 当前版本（符号链接或副本）
│   │   └── ...
│   └── ...
└── _trash/                              # 回收站
    └── {doc_id}/
```

### 4.2 存储策略

- 编辑器内容以 Yjs 编码格式存储（支持增量同步）
- 同时保存一份 HTML/JSON 快照（用于预览和搜索）
- 版本保留最近 N 个（默认 20，可配置）
- 文件存储路径可配置（支持挂载 NAS）

### 4.3 备份

- 支持整目录 rsync 备份
- 数据库 MySQL 标准备份流程
- 版本文件 + 数据库 = 完整恢复

## 5. 前端设计

### 5.1 页面结构

```
┌─────────────────────────────────────────────┐
│ MistDocs 顶部栏                    用户 ▾  │
├──────────┬──────────────────────────────────┤
│          │                                  │
│ 部门树   │  文件列表 / 编辑器区域            │
│          │                                  │
│ ▸ 总公司 │  ┌────────────────────────────┐  │
│   ▸ 技术部│  │ 文档标题     [协作中: 3人]  │  │
│     ▸ 后端│  │                            │  │
│   ▸ 市场部│  │   [编辑器内容区]            │  │
│   ▸ 行政部│  │                            │  │
│          │  │                            │  │
│ 共享文档  │  │                            │  │
│          │  └────────────────────────────┘  │
│ 回收站    │                                  │
├──────────┴──────────────────────────────────┤
│ 状态栏：已保存 | 在线: 3人 | 版本 v5        │
└─────────────────────────────────────────────┘
```

### 5.2 编辑器集成

**文档编辑器（TipTap + Yjs）**：

```
TipTap (ProseMirror)
  ├── y-prosemirror binding
  ├── y-websocket (连接 WS Hub)
  ├── 协同光标插件（显示其他用户位置）
  ├── 扩展：标题、列表、表格、图片、代码块、链接
  └── 导出：HTML / .docx
```

**表格编辑器（Univer）**：

```
Univer
  ├── 自定义 WebSocket 同步（监听单元格变更）
  ├── 公式引擎
  ├── 图表
  ├── 多 Sheet
  └── 导出：.xlsx
```

### 5.3 权限管理界面

- 文件夹/文档右键菜单 → "权限设置"
- 弹窗：添加部门/用户 → 选择权限级别
- 显示当前权限列表，可删除/修改
- 部门管理员只能管本部门资源

## 6. 用户导入设计

### 6.1 CSV 格式

**部门导入**：
```csv
部门名称,上级部门,排序
总公司,,1
技术部,总公司,1
后端组,技术部,1
前端组,技术部,2
市场部,总公司,2
```

**用户导入**：
```csv
姓名,登录名,密码,部门,角色,邮箱,手机
张三,zhangsan,Pass123!,后端组,member,zhangsan@corp.com,13800001111
李四,lisi,Pass123!,技术部,dept_admin,,
```

### 6.2 导入流程

1. 上传 CSV → 解析 → 预览（显示将要导入的数据）
2. 校验：必填项、部门是否存在、登录名是否重复
3. 确认导入 → 批量写入
4. 返回结果（成功数、失败数、失败原因）

### 6.3 可选：MistTerm 同步

```yaml
# config.yaml
mistterm:
  api_url: "http://mistterm:8080/api"
  api_key: "xxx"
```

提供 API 触发一次性同步：
```
POST /api/sync/mistterm
```

同步是可选的、一次性的，不影响独立运行。

## 7. 部署方案

### 7.1 单机部署（推荐）

```
一台服务器（内网，4C8G）
├── mist-docs 二进制（systemd 管理）
├── MySQL 5.7+ / 8.0
├── Nginx（可选，提供 HTTPS）
└── 文件存储 /var/lib/mist-docs/
```

### 7.2 systemd 服务

```ini
[Unit]
Description=MistDocs Server
After=network.target mysql.service

[Service]
Type=simple
User=mist-docs
WorkingDirectory=/opt/mist-docs
ExecStart=/opt/mist-docs/mist-docs -c /etc/mist-docs/config.yaml
Restart=always

[Install]
WantedBy=multi-user.target
```

### 7.3 Nginx 配置

```nginx
server {
    listen 80;
    server_name docs.internal.corp;

    # 前端静态文件
    location / {
        root /opt/mist-docs/web;
        try_files $uri $uri/ /index.html;
    }

    # API
    location /api/ {
        proxy_pass http://127.0.0.1:8900;
    }

    # WebSocket
    location /ws/ {
        proxy_pass http://127.0.0.1:8900;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## 8. 实施计划

| 阶段 | 内容 | 工时 |
|------|------|------|
| Phase 1 | 项目骨架 + 配置 + 数据库迁移 | 2 天 |
| Phase 2 | 用户/部门 CRUD + 登录认证 | 3 天 |
| Phase 3 | 用户 CSV 导入 | 1 天 |
| Phase 4 | 文档 CRUD + 文件存储 + 版本 | 4 天 |
| Phase 5 | 权限中间件 + 权限管理 API | 3 天 |
| Phase 6 | 审计日志 | 2 天 |
| Phase 7 | WebSocket Hub + Yjs 持久化 | 3 天 |
| Phase 8 | 前端骨架 + 文件浏览器 | 3 天 |
| Phase 9 | TipTap 文档编辑器集成 | 3 天 |
| Phase 10 | Univer 表格编辑器集成 | 4 天 |
| Phase 11 | 管理后台（用户/权限/审计） | 3 天 |
| Phase 12 | 测试 + 部署 + 文档 | 2 天 |
| **总计** | | **约 33 天（7 周）** |
