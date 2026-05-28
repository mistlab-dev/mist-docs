# WebSocket 协同协议文档

## 连接

```
ws://host:8900/ws/docs/:doc_id?token=<jwt_token>
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

// 绑定 TipTap
import { ySyncPlugin } from 'y-prosemirror'
// ... TipTap 编辑器配置

// 绑定 Univer
// Univer 需要自定义 WebSocket 同步，通过 Y.Doc + yjs adapter
```

## 注意事项

1. 二进制消息用 `WebSocket.BinaryMessage` 发送
2. JSON 消息用 `WebSocket.TextMessage` 发送
3. Go 服务端只做中转，不解析 Yjs 数据内容
4. 状态持久化每 10 秒自动保存一次
5. 房间无人时自动持久化并销毁
6. 权限在 WebSocket 连接时检查，之后不做实时权限刷新

## 完整前端示例（TipTap + Yjs）

```typescript
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'

// 1. 创建 Yjs 文档
const ydoc = new Y.Doc()

// 2. 连接 WebSocket
const provider = new WebsocketProvider(
  `ws://${location.host}/ws`,
  `docs/${docId}?token=${token}`,
  ydoc
)

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

// 4. 监听同步状态
provider.on('sync', (isSynced: boolean) => {
  console.log('Synced:', isSynced)
})

// 5. 监听在线用户
provider.awareness.on('change', () => {
  const states = provider.awareness.getStates()
  // 更新在线用户列表 UI
})
```

## 完整前端示例（Univer + Yjs）

```typescript
import { Univer, UniverInstanceType } from '@univerjs/core'
import { defaultTheme } from '@univerjs/design'
import { UniverSheetsPlugin } from '@univerjs/sheets'

// Univer 暂不原生支持 Yjs，需要自定义同步层
// 方案：通过 WebSocket 广播单元格变更

// 1. 连接 WebSocket（复用上面的连接）
// 2. 监听 Univer 命令执行
// 3. 将变更序列化发送
// 4. 接收远端变更并应用到本地

// 详细实现待前端开发时补充
```
