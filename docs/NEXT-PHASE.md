# 下一期规划（2026-08-27 更新）

## 已完成总览

| 领域 | 状态 |
|------|------|
| 服务端 4 批功能（AI/审批/Overview/搜索/Monitor/Server Groups） | ✅ 已部署 + 已提交 63130de |
| 片段↔MistDocs 文档联动 | ✅ 已部署 0adeb21 |
| 存储用量聚合 API | ✅ 已部署 2ff80e0 |
| 安全升级 go1.25.13 / npm 0 漏洞 | ✅ 已部署 |
| deploy.sh 安全化 | ✅ 已部署 |
| 客户端待办文档 CLIENT-TODO.md | ✅ 已推 GitHub 4b45ec1 |
| 主站更新到 v1.0.20（下载链接/changelog/版本徽章） | ✅ 已部署 06318c7 + 075ae8e |
| Portal 版本徽章动态拉取 GitHub latest release | ✅ 已部署 10f05a9 |
| 生产 JWT secret 对齐（Portal=MistDocs，SSO 打通） | ✅ 已部署 |
| MistDocs 版本历史功能确认已完整实现 | ✅ 无需额外开发 |

---

## 第三期：产品打磨 + 跨服务扩展（2026-08-27）

> 详见 `PHASE3-PLAN.md`

### A. 官网/Portal 产品打磨

- [x] A1 主站改版（mistlab.dev 新首页、导航、品牌）✅ 已部署
- [x] A2 MistDocs 文档站 UI 统一（主题色对齐）✅ 已部署
- [x] A3 Portal 管理面板美化（设计系统、仪表盘）✅ 已部署
- [x] A4 Pricing 定价页（Free/Pro/Enterprise + 月付年付切换 + FAQ）✅ 已部署
- [x] A5 产品截图展示（首页新增三产品界面 mockup）✅ 已部署
- [x] A6 Onboarding 引导页（新用户 5 步设置流程）✅ 已部署
- [x] C1 MistDocs 公开分享页面（密码保护 + Markdown 渲染）✅ 已部署
- [x] C2 Portal Dashboard 增强（团队概览 + MistDocs 快捷入口）✅ 已部署
- [x] C3 通知系统（实时通知 + Webhook 分发 + SSE 推送）✅ 已部署
- [x] C3b 通知前端（导航栏铃铛 + 面板 + SSE 实时更新）✅ 已部署
- [x] D1 官网 SEO 基础（metadata + JSON-LD + sitemap + robots + Help Center 入口）✅ 已部署
- [x] D2 公开文档中心（Quick start + 安装/团队/审计指南）✅ 已部署
- [x] D3 审计日志导出（CSV/JSON + action/search 筛选）✅ 已部署
- [x] D4 Portal 团队分析（14天活跃趋势 + Snippets/MistDocs/Audit 使用量）✅ 已部署
- [x] D5 移动端体验收尾（官网产品展示 + 审计筛选/导出布局）✅ 已部署

### B. 跨服务功能扩展

- [x] B1 审计报表可视化（前端聚合 + 统计卡 + 趋势图）✅ 已部署
- [x] B2 团队 Snippets 权限细分（admin/editor/viewer + 用户级覆盖）✅ 已部署
- [x] B3 统一管理控制台（Portal 汇总 MistTerm + MistDocs）✅ 已部署

### 优先级

- P0：A1 + B1
- P1：A2 + A3 + B2
- P2：B3

---

---

## 第一阶段：客户端命令审计闭环（P0）

> 阻塞点：需要本地 Rust cargo 环境编译验证

1. [x] 服务器侧审计结果解析验证 — cargo test 11/11 通过 ✅
2. [x] 本地审计文案区分 — 代码已含 "本地检查" / "服务器策略" 文案区分 ✅
3. [x] Agent 不可用降级提示 — 60s 轮询 + 黄色横幅 + 恢复通知 ✅
4. [x] 团队设置页 Agent 状态展示 — 列表/状态/心跳/启禁用 ✅
5. [ ] 实机联调（8 个场景验证）— 等用户手动测试

---

## 第二阶段：片段体验补齐（P1）

6. [x] 异常退出清理编辑锁 — 启动时自动检查 + 清理残留锁 ✅
7. [x] 团队设置页接存储用量 API — 进度条 + 分项展示 + 动态列 ✅

---

## 第三阶段：MistDocs 增强（P2）

8. [x] 文档版本历史 — 已完整实现，无需开发 ✅
9. [x] 文档评论/批注 — TeamCreateComment 修复 user_name 缺失，CRUD 已验证 ✅
10. [x] 文档模板 — 内置故障排查/部署手册/架构设计 3 个模板 ✅

### 涉及仓库
- `/root/work/mist-docs` commit 524556d

---

## 第四阶段：运维安全加固（P2）

11. [x] SSH 密码登录关闭 — PasswordAuthentication no，CA cert 验证通过 ✅
12. [x] 操作审计增强 — audit-archiver.sh 每日归档 SSH/nginx/mist logs + 90天保留 ✅
13. [x] 监控告警 — monitor.sh 每30分钟检查服务/端口/磁盘/内存/负载/OOM/SSH暴力破解 ✅

### 部署记录（2026-08-24）
- sshd_config: PasswordAuthentication no, LogLevel VERBOSE
- crontab: monitor.sh */30 * * * *, audit-archiver.sh 0 2 * * *
- 脚本: /opt/mistlab/monitor.sh, /opt/mistlab/audit-archiver.sh
- 日志: /var/log/mistlab-monitor.log, /var/log/mistlab-alerts.log, /var/log/mistlab-archive/

### 涉及文件
- `scripts/monitor.sh` — 服务/端口/资源监控
- `scripts/audit-archiver.sh` — 日志归档

---

## 验收/发布准则

- 服务端改动：`go build` + `go vet` + `go test`
- 客户端改动：`cargo build` + `cargo test`
- 前端改动：`npm run build` + 页面 200 检查
- 发布前：`git pull --ff-only` + 查最新 tag
- 部署：`./scripts/deploy.sh`（含版本注入 + healthz 验证）

### 涉及文件
- `src/core/cmd_audit.rs` — ServerAuditProbe + 文案
- `src/ui/terminal.rs` — PTY feed + pending_server_audit
- `src/ui/app.rs` — poll_server_audit_from_tabs + handle_server_audit_event
- `src/ui/app_workspace_confirm_modals.rs` — 确认弹窗来源标记
- `src/ui/team_ui.rs` — Agent 状态横幅 + 列表
- `src/core/team/client.rs` — Agent API 调用

### 交付标准
- `cargo build --release` 通过
- `cargo test` 通过
- 8 个场景全部 PASS（见 docs/tech/CLIENT-TODO.md §5）

---

## 第二阶段：片段体验补齐（P1）

6. [x] 异常退出清理编辑锁（启动时检查 + 编辑中心跳续锁 30s + TTL 兜底）
7. [x] 团队设置页接存储用量 API（进度条 + 分项展示）

### 涉及文件
- `src/core/team/service_blocking.rs` — lock/unlock
- `src/ui/team_fragment_dialog.rs` — 编辑界面心跳
- `src/ui/team_ui.rs` — 存储用量 UI

### 交付标准
- `cargo test` 通过
- UI 截图验证

---

## 第三阶段：MistDocs 增强（P2）

8. [x] 文档版本历史（自动保存版本 + diff 查看 + 回滚）— 已完整实现，无需开发
9. [ ] 文档评论/批注（团队成员可评论回复）— 待启动
10. [ ] 文档模板（故障排查、部署手册、架构设计预置模板）— 待启动

### 涉及仓库
- `/root/work/mist-docs`

### 交付标准
- `go build` + `npm run build` 通过
- mist-docs 部署 + 端到端验证

---

## 第四阶段：运维安全加固（P2）

11. [ ] SSH 密码登录关闭（前置：确认所有用户已配置 CA 证书）
12. [ ] 操作审计增强（登录日志、API 调用日志、敏感操作归档）
13. [ ] 监控告警（服务异常 / DB 断连 / 磁盘 >90% 自动告警）

### 交付标准
- 生产机配置变更 + 告警通知验证

---

## 验收/发布准则

- 服务端改动：`go build` + `go vet` + `go test`
- 客户端改动：`cargo build` + `cargo test`
- 前端改动：`npm run build` + 页面 200 检查
- 发布前：`git pull --ff-only` + 查最新 tag
- 部署：`./scripts/deploy.sh`（含版本注入 + healthz 验证）
