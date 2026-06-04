import * as Y from 'yjs'

const MSG_SYNC = 0
const SYNC_STEP1 = 0
const SYNC_STEP2 = 1
const SYNC_UPDATE = 2

// Reconnect limits
const MAX_RECONNECT_DELAY = 30000
const INITIAL_RECONNECT_DELAY = 1000

export interface CollabUser {
  id: string
  name: string
  color: string
}

export class MistWSProvider {
  private ws: WebSocket | null = null
  private doc: Y.Doc
  private url: string
  private connected = false
  private synced = false
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempts = 0
  private destroyed = false

  // Pending updates generated locally while disconnected.
  // Will be sent after reconnection and sync completes.
  private pendingLocalUpdates: Uint8Array[] = []

  public onStatus: ((status: 'connecting' | 'connected' | 'disconnected') => void) | null = null
  public onSynced: ((synced: boolean) => void) | null = null
  public onUserJoin: ((user: CollabUser) => void) | null = null
  public onUserLeave: ((userId: string) => void) | null = null
  public onClients: ((users: CollabUser[]) => void) | null = null

  constructor(url: string, doc: Y.Doc) {
    this.url = url
    this.doc = doc
    this.connect()
  }

  private connect() {
    if (this.destroyed) return
    this.ws = new WebSocket(this.url)
    this.ws.binaryType = 'arraybuffer'
    this.onStatus?.('connecting')
    this.synced = false

    this.ws.onopen = () => {
      this.connected = true
      this.reconnectAttempts = 0
      this.onStatus?.('connected')

      // Sync step 1: send our state vector to request missing updates.
      // Server will respond with step2 containing all updates it has.
      const sv = Y.encodeStateVector(this.doc)
      const msg = new Uint8Array(2 + sv.length)
      msg[0] = MSG_SYNC
      msg[1] = SYNC_STEP1
      msg.set(sv, 2)
      this.ws!.send(msg)
    }

    this.ws.onmessage = (event: MessageEvent) => {
      const data = new Uint8Array(event.data as ArrayBuffer)
      if (!data.length) return

      if (data[0] === MSG_SYNC && data.length >= 2) {
        const subType = data[1]
        const payload = data.slice(2)
        if (subType === SYNC_STEP2 && payload.length > 0) {
          // Apply server's state. Yjs applyUpdate is idempotent —
          // duplicates are automatically handled by CRDT merge logic.
          Y.applyUpdate(this.doc, payload, this)
          
          // Mark as synced and flush any pending local updates.
          if (!this.synced) {
            this.synced = true
            this.onSynced?.(true)
            this.flushPendingUpdates()
          }
        } else if (subType === SYNC_UPDATE && payload.length > 0) {
          // Apply remote update
          Y.applyUpdate(this.doc, payload, this)
        }
      } else {
        // JSON messages (join/leave/clients/awareness)
        try {
          const text = new TextDecoder().decode(data)
          const msg = JSON.parse(text)
          if (msg.type === 'join') this.onUserJoin?.(msg.user)
          else if (msg.type === 'leave') this.onUserLeave?.(msg.user.id)
          else if (msg.type === 'clients') this.onClients?.(msg.users)
        } catch {}
      }
    }

    this.ws.onclose = () => {
      this.connected = false
      this.synced = false
      this.onStatus?.('disconnected')
      this.onSynced?.(false)
      this.scheduleReconnect()
    }

    this.ws.onerror = () => {
      // onclose will be called after this, triggering reconnect
      this.ws?.close()
    }
  }

  private scheduleReconnect() {
    if (this.destroyed || this.reconnectTimer) return
    // Exponential backoff: 1s, 2s, 4s, ... up to 30s
    const delay = Math.min(INITIAL_RECONNECT_DELAY * Math.pow(2, this.reconnectAttempts), MAX_RECONNECT_DELAY)
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.reconnectAttempts++
      this.connect()
    }, delay)
  }

  // handleUpdate captures local edits and sends to server.
  // If not connected/synced, buffers in pendingLocalUpdates for later.
  private handleUpdate = (update: Uint8Array, origin: any) => {
    if (origin === this) return // ignore updates we applied from server
    
    if (!this.connected || !this.synced) {
      // Buffer locally until reconnection
      this.pendingLocalUpdates.push(update)
      return
    }
    
    this.sendUpdate(update)
  }

  private sendUpdate(update: Uint8Array) {
    const msg = new Uint8Array(2 + update.length)
    msg[0] = MSG_SYNC
    msg[1] = SYNC_UPDATE
    msg.set(update, 2)
    try { this.ws?.send(msg) } catch {}
  }

  // flushPendingUpdates sends any buffered local edits after sync completes.
  private flushPendingUpdates() {
    if (!this.connected || this.pendingLocalUpdates.length === 0) return
    for (const update of this.pendingLocalUpdates) {
      this.sendUpdate(update)
    }
    this.pendingLocalUpdates = []
  }

  bind() {
    this.doc.on('update', this.handleUpdate)
  }

  unbind() {
    this.doc.off('update', this.handleUpdate)
  }

  sendAwareness(data: any) {
    if (!this.connected) return
    const msg = JSON.stringify({ type: 'awareness', data })
    try { this.ws?.send(new TextEncoder().encode(msg)) } catch {}
  }

  destroy() {
    this.destroyed = true
    this.unbind()
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
    this.ws = null
    this.pendingLocalUpdates = []
  }

  // Getter for external status checks
  isConnected(): boolean {
    return this.connected
  }

  isSynced(): boolean {
    return this.synced
  }
}