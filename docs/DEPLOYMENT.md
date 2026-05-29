# MistDocs 部署指南

## 目录

- [Docker 部署（推荐）](#docker-部署推荐)
- [手动部署](#手动部署)
- [Nginx 反向代理](#nginx-反向代理)
- [环境变量参考](#环境变量参考)
- [数据库说明](#数据库说明)
- [备份与恢复](#备份与恢复)
- [常见问题](#常见问题)

---

## Docker 部署（推荐）

### 前置条件

- Docker 20.10+
- Docker Compose v2

### 1. 获取代码

```bash
git clone https://github.com/c-wind/mist-docs.git
cd mist-docs
```

### 2. 配置环境变量

```bash
cp .env.example .env
vim .env
```

必改项：

```env
# MySQL 密码（生产环境务必修改）
MYSQL_ROOT_PASSWORD=your_root_password
MYSQL_PASSWORD=your_app_password

# JWT 密钥（生产环境务必修改）
JWT_SECRET=your-random-secret-key
```

### 3. 一键启动

```bash
docker compose up -d
```

### 4. 查看日志

```bash
docker compose logs -f app
```

### 5. 访问

浏览器打开 `http://your-server:8900`

### 6. 创建管理员

首次启动需要创建管理员账户：

```bash
# 方式一：通过 API
curl -X POST http://localhost:8900/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "username": "admin",
    "password": "YourStrongPassword!",
    "name": "管理员"
  }'
```

### 自定义配置文件

如需完全自定义配置，挂载自己的 `config.yaml`：

```yaml
# docker-compose.yml
services:
  app:
    volumes:
      - ./my-config.yaml:/app/configs/config.yaml
      - app_data:/data
    environment:
      GENERATE_CONFIG: "no"  # 禁用自动生成
```

### 更新版本

```bash
git pull
docker compose up -d --build
```

---

## 手动部署

### 前置条件

- Go 1.21+
- Node.js 18+（前端构建）
- MySQL 8.0+
- Linux 系统（CentOS 7+ / Ubuntu 20.04+）

### 1. 编译后端

```bash
git clone https://github.com/c-wind/mist-docs.git
cd mist-docs
go build -o mist-docs ./cmd/server/
```

### 2. 编译前端

```bash
cd web
npm install
npm run build
cd ..
```

### 3. 准备数据库

```bash
# 登录 MySQL
mysql -u root -p

# 创建数据库和用户
CREATE DATABASE mist_docs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'mist_docs'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON mist_docs.* TO 'mist_docs'@'localhost';
FLUSH PRIVILEGES;

# 导入表结构
mysql -u mist_docs -p mist_docs < docker/init-db.sql
```

### 4. 配置

```bash
mkdir -p /etc/mist-docs
cp configs/config.yaml /etc/mist-docs/config.yaml
vim /etc/mist-docs/config.yaml
```

修改数据库连接、JWT 密钥等。

### 5. 部署文件

```bash
# 二进制
cp mist-docs /usr/local/bin/

# 前端静态文件
mkdir -p /var/www/mistdocs/web
cp -r web/dist/* /var/www/mistdocs/web/

# 数据目录
mkdir -p /var/lib/mist-docs/files
```

### 6. 创建 systemd 服务

```bash
cat > /etc/systemd/system/mist-docs.service << 'EOF'
[Unit]
Description=MistDocs Document Management
After=network.target mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=/var/www/mistdocs
ExecStart=/usr/local/bin/mist-docs -c /etc/mist-docs/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable mist-docs
systemctl start mist-docs
```

### 7. 验证

```bash
systemctl status mist-docs
curl http://localhost:8900/api/auth/me
```

---

## Nginx 反向代理

### 基本配置

```nginx
server {
    listen 80;
    server_name docs.example.com;

    # 强制 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name docs.example.com;

    ssl_certificate     /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;

    client_max_body_size 50M;

    location / {
        proxy_pass http://127.0.0.1:8900;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket
    location /ws/ {
        proxy_pass http://127.0.0.1:8900;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }

    # 静态文件缓存
    location /assets/ {
        proxy_pass http://127.0.0.1:8900;
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

### Docker 部署 + Nginx

```nginx
location / {
    proxy_pass http://127.0.0.1:8900;
    # ... 同上
}
```

Nginx 装在宿主机，反代到 Docker 映射的 8900 端口。

---

## 环境变量参考

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GENERATE_CONFIG` | `yes` | 是否从环境变量生成配置 |
| `DB_HOST` | `mysql` | MySQL 主机 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | `mist_docs` | MySQL 用户 |
| `DB_PASS` | `mist_docs_pass` | MySQL 密码 |
| `DB_NAME` | `mist_docs` | 数据库名 |
| `SERVER_PORT` | `8900` | 服务监听端口 |
| `JWT_SECRET` | `mist-docs-jwt-...` | JWT 签名密钥 |
| `DATA_DIR` | `/data` | 文件存储目录 |
| `LOG_LEVEL` | `info` | 日志级别 |
| `MAX_FILE_SIZE` | `52428800` | 最大文件大小（50MB） |
| `VERSION_KEEP` | `20` | 版本保留数量 |
| `WS_MAX_MSG` | `1048576` | WebSocket 最大消息（1MB） |
| `WS_PING_INTERVAL` | `30` | WebSocket 心跳间隔（秒） |

---

## 数据库说明

### 表清单

| 表名 | 说明 |
|------|------|
| `md_departments` | 部门 |
| `md_users` | 用户 |
| `md_folders` | 文件夹 |
| `md_documents` | 文档 |
| `md_versions` | 文档版本 |
| `md_permissions` | 权限 |
| `md_audits` | 审计日志 |
| `md_keys` | 加密密钥 |
| `md_shares` | 分享链接 |
| `md_comments` | 评论 |
| `md_notifications` | 通知 |
| `md_tags` | 标签 |
| `md_doc_tags` | 文档标签关联 |
| `md_favorites` | 收藏 |
| `md_webhooks` | Webhook 配置 |
| `md_webhook_logs` | Webhook 投递日志 |

### 表前缀

所有表以 `md_` 为前缀，可与其他应用共享同一个数据库。

---

## 备份与恢复

### Docker 环境

```bash
# 备份数据库
docker compose exec mysql mysqldump -u mist_docs -p'password' mist_docs > backup_$(date +%Y%m%d).sql

# 备份文件
docker run --rm -v mist-docs_app_data:/data -v $(pwd):/backup alpine \
  tar czf /backup/files_$(date +%Y%m%d).tar.gz -C /data .

# 恢复数据库
docker compose exec -T mysql mysql -u mist_docs -p'password' mist_docs < backup.sql

# 恢复文件
docker run --rm -v mist-docs_app_data:/data -v $(pwd):/backup alpine \
  sh -c "cd /data && tar xzf /backup/files.tar.gz"
```

### 手动部署

```bash
# 备份
mysqldump -u mist_docs -p mist_docs > backup.sql
tar czf files_backup.tar.gz /var/lib/mist-docs/files/

# 恢复
mysql -u mist_docs -p mist_docs < backup.sql
tar xzf files_backup.tar.gz -C /
```

### 自动备份（cron）

```bash
# 每天凌晨 3 点备份
0 3 * * * /usr/bin/docker compose -f /opt/mist-docs/docker-compose.yml exec -T mysql mysqldump -u mist_docs -p'password' mist_docs | gzip > /backup/mist_docs_$(date +\%Y\%m\%d).sql.gz
```

---

## 常见问题

### Q: 启动报数据库连接失败

检查 MySQL 是否就绪：

```bash
# Docker
docker compose logs mysql

# 手动
mysql -u mist_docs -p -h 127.0.0.1 -e "SELECT 1"
```

Docker 环境中 app 会自动等待 MySQL 就绪（最多 60 秒）。

### Q: 上传文件失败 413

Nginx 默认限制 1MB，需要修改：

```nginx
client_max_body_size 50M;
```

### Q: WebSocket 连不上

确保 Nginx 配置了 WebSocket 代理（见上方 Nginx 配置）。

### Q: 忘记管理员密码

```bash
# Docker
docker compose exec mysql mysql -u root -p
# 手动
mysql -u root -p

# 重置密码（密码会被重新哈希）
UPDATE md_users SET password='$2a$10$new_hash' WHERE username='admin';
```

### Q: 如何切换数据库

默认使用 MySQL。配置文件中修改 `database` 部分即可指向不同的 MySQL 实例。暂不支持其他数据库。

### Q: Docker 构建慢

首次构建需要下载 Go/Node 依赖，约 5-10 分钟。后续利用缓存会快很多。

```bash
# 仅重建应用（跳过依赖下载）
docker compose build --no-cache app
```

### Q: 如何查看应用日志

```bash
# Docker
docker compose logs -f app

# 手动部署
journalctl -u mist-docs -f
```
