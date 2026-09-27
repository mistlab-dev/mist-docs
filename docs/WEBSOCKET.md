# WebSocket 协同协议文档

## 连接

富文本文档使用这条连接。表格不走 Yjs。

`token` 是登录后存在 `localStorage` 键 `mist-docs-token` 里的 Portal JWT。8900 只在本机；浏览器连的是当前站点的 `/ws/...`。

```
ws(s)://<host>/ws/teams/:team_id/docs/:doc_id?token=<portal_jwt>
```

## 消息类型

### 二进制消息（Yjs 同步协议）

格式：`[msgType, subType, ...payload]`

| msgType | subType | 说明 |
|---------|---------|------|
| 0 (sync) | 0 (step1) | 客户端发送 state vector，请求缺失的更新 |
| 0 (sync) | 1 (step2) | 发送差异更新（包含 state 或 diff） |
| 0 (sync) | 2 (update) | 增量更新（每次编辑操作） |

### JSON 消息

JSON 消息也用**二进制帧**发送（UTF-8 编码的 JSON）。第一个字节不是 `0`（sync）的帧按 JSON 解析，见 `web/src/utils/collab.ts`。

**客户端 → 服务端：**

```json
{"type": "awareness", "data": {...}}
```

**服务端 → 客户端：**

```json
// 新用户加入
{"type": "join", "user": {"id": "xxx", "name": "张三", "color": "#e06c75"}}

// 用户离开
{"type": "leave", "user": {"id": "xxx"}}

// 当前在线用户列表（加入时发送）
{"type": "clients", "users": [{"id": "xxx", "name": "张三", "color": "#e06c75"}, ...]}

// 其他人的光标/选区
{"type": "awareness", "user_id": "xxx", "data": {...}}

// 写权限变化（有人加锁/解锁、协作者权限被改）。编辑器据此切换只读
{"type": "permission", "can_write": false, "locked_by": "u_123", "locked_by_name": "张三"}

// 访问权被收回，随后服务端以关闭码 4403 断开，客户端不再重连
{"type": "access", "reason": "revoked"}
```

## 权限与锁

- **连接时**校验：团队成员、文档属于该团队且未删除、至少有读权限。
- **定期复查**：每 20 秒按团队角色和文档权限再查一次；另外，距上次检查超过 5 秒时，服务端在接受一条编辑前会先复查。被移出团队、权限被收回或文档被删除时，发送 `access` 消息并以关闭码 **4403** 断开。
- **即时复查**：HTTP 接口加锁、解锁、修改协作者权限后，服务端立即复查该文档的所有连接（`handler.OnDocAccessChanged` → `Hub.Reauthorize`）。
- **只读连接**：查看者，或文档被他人锁定时（管理员除外），连接保持、可以跟上同步，但服务端会丢掉他们发出的文档更新，并用 `permission` 消息告诉客户端 `can_write: false` 和锁定人。
- HTTP 侧：`POST /documents/:id/lock` 被他人占用时返回 409 和 `locked_by` / `locked_by_name`；文档详情也带这两个字段。

## 前端集成（y-websocket）

```javascript
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'

const doc = new Y.Doc()
const wsProvider = new WebsocketProvider(
  `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/teams/${teamId}`,
  `docs/${docId}?token=${token}`,
  doc,
  { WebSocketPolyfill: ... }
)

// 富文本使用 web/src/utils/collab.ts 里的 MistWSProvider。
// 表格编辑器不连接这条 WebSocket。
```

## 注意事项

1. 所有消息（Yjs 与 JSON）都用 `WebSocket.BinaryMessage` 发送
2. 客户端需设置 `binaryType = 'arraybuffer'`
3. Go 服务端只做中转，不解析 Yjs 数据内容
4. 状态持久化每 10 秒自动保存一次
5. 房间无人时自动持久化并销毁
6. 权限、锁和断开规则见上面的“权限与锁”。

## 完整前端示例（TipTap + Yjs）

实际连接在 `web/src/views/DocEditor.vue`：只在文档类型为 `doc` 且 `mist-docs-token` 非空时创建 `MistWSProvider`。

```typescript
import * as Y from 'yjs'
import { MistWSProvider } from '@/utils/collab'

const ydoc = new Y.Doc()
const token = localStorage.getItem('mist-docs-token')
const wsUrl = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/ws/teams/${teamId}/docs/${docId}?token=${token}`
const provider = new MistWSProvider(wsUrl, ydoc)
```

下面是 TipTap 绑定的形状（与编辑器里的 Collaboration 扩展一致）：

```typescript
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'

// 3. 创建 TipTap 编辑器
const editor = new Editor({
  extensions: [
    StarterKit.configure({
      history: false, // Yjs 管理历史
    }),
    Collaboration.configure({
      document: ydoc,
    }),
    CollaborationCursor.configure({
      provider,
      user: {
        name: userName,
        color: userColor,
      },
    }),
  ],
})

// MistWSProvider 用回调，不是 y-websocket 的 awareness API。
provider.onStatus = (status) => { /* connecting | connected | disconnected */ }
provider.onSynced = (synced) => { /* 首次同步完成 */ }
provider.onClients = (users) => { /* 当前在线用户 */ }
```

## 表格

表格使用 `web/src/components/SheetEditor.vue`。它不连接 Yjs。改完保存，其他人刷新后看到新内容。
