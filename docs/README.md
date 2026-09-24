# MistDocs 快速入门

## 30 秒部署

```bash
git clone https://github.com/mistlab-dev/mist-docs.git
cd mist-docs
cp .env.example .env
# 编辑 .env 修改数据库密码和 JWT 密钥
docker compose up -d
```

本地 Docker 监听 8900。生产入口是 https://docs.mistlab.dev ，8900 只在服务器本机。登录走 mistlab.dev Portal，不要在本服务里建管理员。

---

## 功能概览

### 📄 文档管理
- 富文本编辑器（TipTap）
- 文件夹树形管理
- 文档移动、批量操作
- 文档锁定/解锁
- 自动保存 + 保存状态指示

### 📥 导入
- .txt / .md / .html → 文档
- .docx（Word）→ 文档，保留标题层级
- .xlsx（Excel）→ 表格

### 📤 导出
- HTML / Markdown / 纯文本 / PDF（扩展名与内容一致）
- 不提供 Word（.docx / .doc）下载。`.docx` 只用于导入
- 编辑器里的 PDF 由浏览器生成，不走服务端导出

### 📊 表格
- 自研 `SheetEditor.vue`（公式、图表、数据透视）
- 公式以编辑器里的 `evalFormula` 为准，没有 SUMIF、COUNTIF、IFS、SWITCH：
  - 聚合：SUM / AVERAGE（AVG）/ COUNT / COUNTA / MAX / MIN
  - 数学：ROUND / CEILING（CEIL）/ FLOOR / ABS / MOD / POWER（POW）/ SQRT / LOG / LOG10 / EXP / PI / INT / RAND / RANDBETWEEN
  - 文本：CONCAT（CONCATENATE）/ LEFT / RIGHT / MID / LEN（LENGTH）/ UPPER / LOWER / TRIM / SUBSTITUTE（REPLACE）/ TEXT / VALUE
  - 日期：NOW / TODAY / YEAR / MONTH / DAY / DATEDIF / WEEKDAY
  - 逻辑与判断：IF / AND / OR / NOT / ISBLANK / ISNUMBER / ISTEXT
  - 查找：VLOOKUP / INDEX / MATCH / CHOOSE
- 图表（canvas）和数据透视
- 表格不走 Yjs。保存后，其他人刷新才能看到

### 🏷️ 标签系统
- 创建/删除标签（带颜色）
- 按标签筛选文档

### 📋 文档模板
- 空白 / 会议纪要 / 周报 / 需求文档 / API文档 / README / 故障排查 SOP / 变更运维手册（Runbook）

### 🔄 版本管理
- 自动保存版本历史
- 版本对比（diff 红绿高亮）
- 一键回退

### 📊 文档统计
- 字数 / 字符数 / 编辑次数
- 贡献者列表
- 活跃时段图表

### 🔗 协作
- 富文本文档：登录后走 Yjs WebSocket（`/ws/teams/:team_id/docs/:doc_id`）
- 表格：保存后刷新，没有实时协同
- 评论（支持回复 + @提及，约 10 秒轮询）
- 文档分享（密码 + 过期时间）。角色是查看者 / 编辑者 / 管理员

### 🧹 回收站
- 软删除 + 恢复 + 永久删除 + 清空
- 不会按 30 天自动清理

### 🌐 Webhook
- 文档变更通知外部系统
- 默认订阅 `document.created` 与 `document.updated`
- 创建文档投递 `document.created`，保存文档投递 `document.updated`（审计动作 `create_doc` / `edit_doc` 会映射到这两个名字）
- 投递日志 + 开关控制

### 🔐 权限
- **团队隔离**：所有业务数据按 `team_id` 划分（`/api/teams/:team_id/**`）
- 团队成员校验由中间件强制（非成员 → 403）
- 角色控制（admin / editor / viewer）
- 文件夹 ACL + 文档级分享（显式分享优先）

### 🔗 生态联动
- 与 MistTerm 团队片段双向联动（文档 ↔ 片段绑定）
- 段落级知识检索（供 MistTerm AI 拉取知识）
- 批量执行记录一键沉淀为文档

### 🎨 其他
- 深色模式
- 键盘快捷键
- 大纲导航（标题跳转）
- 水印
- 移动端适配
- API 限流（30 req/s）
- OpenAPI 3.0 文档

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go + Gin |
| 前端 | Vue 3 + Element Plus + TipTap |
| 数据库 | MySQL 8.0 |
| 协作 | WebSocket (Gorilla) |
| 加密 | AES-256-GCM |
| 部署 | Docker / systemd |

---

## 项目结构

```
mist-docs/
├── cmd/server/          # 入口
├── internal/
│   ├── config/          # 配置
│   ├── database/        # 数据库
│   ├── handler/         # HTTP handlers
│   ├── middleware/       # 中间件（JWT/CORS/限流）
│   ├── model/           # 数据模型
│   ├── service/         # 业务逻辑
│   ├── store/           # 文件存储
│   ├── ws/              # WebSocket
│   └── crypto/          # 加密
├── web/                 # 前端（Vue 3）
├── migrations/          # 数据库迁移
├── docker/              # Docker 相关文件
├── docs/                # 文档
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

---

## API 文档

本机进程：`http://127.0.0.1:8900/api/openapi.json`。`POST /auth/login` 已废弃，登录走 Portal。路径前缀是 `/api/teams/{team_id}/...`。

或在线查看：导入到 [Swagger Editor](https://editor.swagger.io)

---

## 相关链接

- [部署指南](DEPLOYMENT.md)
- [设计文档](DESIGN.md)
- [WebSocket 协议](WEBSOCKET.md)
- [GitHub](https://github.com/mistlab-dev/mist-docs)
