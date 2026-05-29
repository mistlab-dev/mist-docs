# MistDocs Docker 部署

## 快速启动

```bash
# 1. 复制配置
cp .env.example .env
# 编辑 .env 修改密码等配置
vim .env

# 2. 一键启动
docker compose up -d

# 3. 查看日志
docker compose logs -f app

# 4. 访问
# http://your-server:8900
```

## 默认账号

首次启动后通过 API 创建管理员：

```bash
# 注册管理员
curl -X POST http://localhost:8900/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@2026","name":"管理员"}'
```

或直接在数据库插入后修改密码。

## 数据持久化

数据存储在 Docker volumes 中：
- `mysql_data` — MySQL 数据
- `app_data` — 文档文件、上传文件

```bash
# 查看数据卷
docker volume ls

# 备份数据
docker exec mist-docs-mysql-1 mysqldump -u mist_docs -p'mist_docs_pass' mist_docs > backup.sql
```

## 自定义配置

如需完全自定义配置文件，可挂载：

```yaml
# docker-compose.yml
app:
  volumes:
    - ./my-config.yaml:/app/configs/config.yaml
    - app_data:/data
  environment:
    GENERATE_CONFIG: "no"  # 禁用自动生成
```

## 常用命令

```bash
# 启动
docker compose up -d

# 停止
docker compose down

# 重新构建（代码更新后）
docker compose up -d --build

# 查看日志
docker compose logs -f app

# 进入容器
docker compose exec app sh

# 重启
docker compose restart app
```

## 生产部署建议

1. 修改 `.env` 中的 `JWT_SECRET` 和 `MYSQL_PASSWORD`
2. 在前面加 Nginx/Caddy 反代 + HTTPS
3. MySQL 不对外暴露端口（去掉 `MYSQL_PORT_MAP`）
4. 定期备份 MySQL 数据
