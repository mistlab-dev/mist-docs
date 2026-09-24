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

**服务端 → 客户端：**

```json
// 新用户加入
{"type": "join", "user": {"id": "xxx", "name": "张三", "color": "#e06c75"}}

// 用户离开
{"type": "leave", "user": {"id": "xxx"}}

// 当前在线用户列表（加入时发送）
{"type": "clients", "users": [{"id": "xxx", "name": "张三", "color": "#e06c75"}, ...]}
```

## 前端集成（y-websocket）

```javascript
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'

const doc = new Y.Doc()
const wsProvider = new WebsocketProvider(
  'ws://host:8900/ws',
  `docs/${docId}?token=${token}`,
  doc,
  { WebSocketPolyfill: ... }
)

// 富文本使用 web/src/utils/collab.ts 里的 MistWSProvider。
// 表格编辑器不连接这条 WebSocket。
```

## 注意事项

1. 二进制消息用 `WebSocket.BinaryMessage` 发送
2. JSON 消息用 `WebSocket.TextMessage` 发送
3. Go 服务端只做中转，不解析 Yjs 数据内容
4. 状态持久化每 10 秒自动保存一次
5. 房间无人时自动持久化并销毁
6. 连接时校验团队成员、文档归属和读权限。查看者可以跟上同步，服务端会丢掉他们发出的文档更新。之后每 60 秒按团队角色再查一次，权限被收回就断开。

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
