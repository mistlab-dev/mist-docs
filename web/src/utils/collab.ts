import * as Y from 'yjs'

const MSG_SYNC = 0
const SYNC_STEP1 = 0
const SYNC_STEP2 = 1
const SYNC_UPDATE = 2

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
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempts = 0
  private destroyed = false

  public onStatus: ((status: 'connecting' | 'connected' | 'disconnected') => void) | null = null
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

    this.ws.onopen = () => {
      this.connected = true
      this.reconnectAttempts = 0
      this.onStatus?.('connected')

      // Send sync step 1: our state vector
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
          Y.applyUpdate(this.doc, payload)
        } else if (subType === SYNC_UPDATE && payload.length > 0) {
          Y.applyUpdate(this.doc, payload)
        }
      } else {
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
      this.onStatus?.('disconnected')
      this.scheduleReconnect()
    }

    this.ws.onerror = () => {
      this.ws?.close()
    }
  }

  private scheduleReconnect() {
    if (this.destroyed || this.reconnectTimer) return
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000)
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.reconnectAttempts++
      this.connect()
    }, delay)
  }

  private handleUpdate = (update: Uint8Array, origin: any) => {
    if (origin === this || !this.connected) return
    const msg = new Uint8Array(2 + update.length)
    msg[0] = MSG_SYNC
    msg[1] = SYNC_UPDATE
    msg.set(update, 2)
    try { this.ws?.send(msg) } catch {}
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
  }
}
