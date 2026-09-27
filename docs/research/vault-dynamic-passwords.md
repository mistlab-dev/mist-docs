# Vault 服务器账号动态密码调研

## 一、背景

传统服务器账号管理痛点：
- 多台机器多个账号，密码难以统一管理
- 密码轮换靠人工，容易遗漏
- 共享密码泄露后影响范围大
- 人员离职后密码回收不及时

Vault 的动态密码（Dynamic Secrets）可以解决这些问题。

---

## 二、Vault SSH Secrets Engine 两种模式

### 1. OTP 模式（简单但局限）

- Vault 生成一次性密码，服务器装 vault-helper 验证
- 只支持密码认证，不推荐

### 2. CA 签名模式（✅ 已采用）

- Vault 作为 SSH Certificate Authority (CA)
- 签发短期证书，自动过期，无需撤销
- 服务器只需配一行 TrustedUserCAKeys
- 行业标准方案（Netflix、Uber 在用）

---

## 三、Vault 租户架构（已实施）

每个团队独立 SSH secrets engine + 独立 CA + 独立策略：

```
Vault Server (开发机 <dev-host>:8200)
├── ssh/                    # 全局 CA（生产机、运维）
├── team-backend/ssh/       # 后端团队独立 CA
├── team-frontend/ssh/      # 前端团队独立 CA
└── team-devops/ssh/        # 运维团队独立 CA（跨团队权限）
```

**隔离机制：**
- 每个团队独立 CA 密钥对（指纹不同）
- Vault Policy 限制只能访问自己团队的 path
- 团队用户登录后签发的是自己团队的证书
- 不同团队证书互不通用

**权限设计：**

| 团队 | 角色 | 用户 | 可登录用户 |
|------|------|------|-----------|
| team-backend | admin, readonly | backend-admin, backend-dev | root, deploy, readonly |
| team-frontend | admin, readonly | frontend-admin | root, deploy, readonly |
| team-devops | admin | devops-admin | *（任意用户）+ 全局 ssh |

### 添加新团队

```bash
# 1. 启用新团队的 secrets engine
vault secrets enable -path=team-newteam/ssh ssh
vault write -f team-newteam/ssh/config/ca generate_signing_key=true

# 2. 创建角色
vault write team-newteam/ssh/roles/admin \
  key_type=ca algorithm_signer=rsa-sha2-256 \
  allowed_users="root,deploy" default_user=root ttl=4h max_ttl=24h

# 3. 创建策略
cat <<'EOF' | vault policy write team-newteam -
path "team-newteam/ssh/*" { capabilities = ["read", "list"] }
path "team-newteam/ssh/sign/*" { capabilities = ["create", "read", "update"] }
EOF

# 4. 创建用户
vault write auth/userpass/users/newteam-admin \
  password="..." policies="team-newteam"
```

### 服务器配置（信任特定团队 CA）

```bash
# 只允许 backend 团队登录
echo "TrustedUserCAKeys /etc/ssh/team-backend-ca.pub" >> /etc/ssh/sshd_config

# 允许多个团队（逗号分隔）
echo "TrustedUserCAKeys /etc/ssh/team-backend-ca.pub,/etc/ssh/team-frontend-ca.pub" >> /etc/ssh/sshd_config
```

---

## 四、部署状态

### ✅ 已完成

- Vault 1.18.2 二进制安装
- Raft 存储模式，systemd 管理，开机自启
- 全局 SSH CA + 3 个团队独立 CA
- 开发机 sshd 信任全局 CA
- userpass 认证方式
- 团队隔离验证通过
- 管理脚本 `/usr/local/bin/vault-team.sh`

### ⏳ 待完成

- [ ] 生产机 <prod-host> 配置 CA 信任
- [ ] 密钥文件备份到安全位置
- [ ] Vault 审计日志开启
- [ ] 数据库动态密码（MySQL）

---

## 五、关键文件

| 文件 | 说明 |
|------|------|
| `/etc/vault.d/vault.hcl` | Vault 配置 |
| `/etc/vault.d/vault.service` | systemd 服务 |
| `/root/vault-keys.json` | Root Token + Unseal Keys（权限 600，仅存于运维机本地，不入库） |
| `/etc/vault.d/ssh-ca-public.pub` | 全局 CA 公钥 |
| `/etc/vault.d/team-*-ca.pub` | 团队 CA 公钥 |
| `/etc/ssh/ca.pub` | 开发机信任的 CA 公钥 |
| `/usr/local/bin/vault-team.sh` | 团队管理脚本 |

---

## 六、参考资料

- Vault SSH Secrets Engine：https://developer.hashicorp.com/vault/docs/secrets/ssh
- Vault 动态数据库凭证：https://developer.hashicorp.com/vault/docs/secrets/databases
