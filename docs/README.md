# MistDocs 快速入门

## 30 秒部署

```bash
git clone https://github.com/c-wind/mist-docs.git
cd mist-docs
cp .env.example .env
# 编辑 .env 修改数据库密码和 JWT 密钥
docker compose up -d
```

打开 `http://your-server:8900`，完成。

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
- .xlsx（Excel）→ 智能表格

### 📤 导出
- PDF / HTML / Markdown / 纯文本 / Word(.doc)

### 📊 智能表格
- 在线表格编辑器
- **60+ 公式函数**：
  - 聚合：SUM/AVG/COUNT/MAX/MIN/SUMIF/COUNTIF
  - 数学：ABS/ROUND/CEIL/FLOOR/POWER/MOD/SQRT/LOG
  - 文本：CONCAT/LEN/LEFT/RIGHT/UPPER/LOWER/TRIM
  - 日期：NOW/TODAY/YEAR/MONTH/DAY/DATEDIF
  - 逻辑：IF/IFS/AND/OR/NOT/SWITCH
  - 查找：VLOOKUP/INDEX/MATCH/CHOOSE
- 图表（柱状图/折线图/饼图）

### 🏷️ 标签系统
- 创建/删除标签（带颜色）
- 按标签筛选文档

### 📋 文档模板
- 空白 / 会议纪要 / 周报 / 需求文档 / API文档 / README

### 🔄 版本管理
- 自动保存版本历史
- 版本对比（diff 红绿高亮）
- 一键回退

### 📊 文档统计
- 字数 / 字符数 / 编辑次数
- 贡献者列表
- 活跃时段图表

### 🔗 协作
- WebSocket 实时协作
- 评论（支持回复 + @提及）
- 文档分享（密码保护 + 过期时间）

### 🧹 回收站
- 软删除 + 恢复 + 永久删除

### 🌐 Webhook
- 文档变更通知外部系统
- 支持 create/update/delete/restore 事件
- 投递日志 + 开关控制

### 🔐 权限
- 部门隔离
- 角色控制（超管 / 部门管理员 / 普通成员）
- 文档级权限

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

启动后访问：`http://your-server:8900/api/openapi.json`

或在线查看：导入到 [Swagger Editor](https://editor.swagger.io)

---

## 相关链接

- [部署指南](DEPLOYMENT.md)
- [设计文档](DESIGN.md)
- [WebSocket 协议](WEBSOCKET.md)
- [GitHub](https://github.com/c-wind/mist-docs)
