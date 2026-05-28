# mist-docs

企业文档协同服务 —— MistTerm 生态的独立文档模块。

## 定位

- 独立部署、独立运行的文档服务
- 自带用户/部门/权限管理
- 支持从 MistTerm 或 CSV 导入用户数据
- 实时协同编辑（文档 + 表格）
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

## 项目结构

```
mist-docs/
├── cmd/server/          # 入口
├── internal/
│   ├── model/           # 数据模型
│   ├── handler/         # HTTP Handler
│   ├── middleware/       # JWT 鉴权 + 权限检查
│   ├── ws/              # WebSocket Hub（协同中转）
│   └── store/           # 文件存储
├── web/                 # 前端
│   ├── src/
│   │   ├── views/       # 页面
│   │   ├── components/  # 编辑器组件
│   │   └── stores/      # 状态管理
│   └── public/
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

## 与 MistTerm 的关系

- 独立项目，不依赖 MistTerm 运行
- 可通过 API 从 MistTerm 同步用户/部门数据
- 共享相同的认证方案（JWT），可选择性打通单点登录
