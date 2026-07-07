import type { SignalingEnvelope, UserSession } from '@/types'

type MessageHandler = (message: SignalingEnvelope) => void
type StatusHandler = (status: 'connecting' | 'connected' | 'closed' | 'error', message?: string) => void

export class SignalingClient {
  private socket: WebSocket | null = null
  private pingTimer: number | null = null

  constructor(
    private readonly roomId: string,
    private readonly user: UserSession,
    private readonly livekitIdentity: string,
    private readonly onMessage: MessageHandler,
    private readonly onStatus: StatusHandler,
  ) {}

  connect(appWsUrl: string) {
    this.onStatus('connecting')
    const url = new URL(appWsUrl)
    url.searchParams.set('userId', String(this.user.userId))
    url.searchParams.set('username', this.user.username)
    this.socket = new WebSocket(url.toString())
    this.socket.onopen = () => {
      this.onStatus('connected')
      this.send('room.join', {
        clientType: 'web',
        displayName: this.user.username,
        livekitIdentity: this.livekitIdentity,
      })
      this.pingTimer = window.setInterval(() => {
        this.send('ping', {})
      }, 20000)
    }
    this.socket.onmessage = (event) => {
      this.onMessage(JSON.parse(event.data) as SignalingEnvelope)
    }
    this.socket.onclose = () => {
      this.clearPing()
      this.onStatus('closed')
    }
    this.socket.onerror = () => {
      this.onStatus('error', 'WebSocket 连接失败')
    }
  }

  send(type: string, payload: unknown, peerId = '') {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return
    }
    const envelope: SignalingEnvelope = {
      type,
      requestId: `req-${Date.now()}-${Math.random().toString(16).slice(2)}`,
      roomId: this.roomId,
      peerId,
      userId: this.user.userId,
      username: this.user.username,
      payload,
    }
    this.socket.send(JSON.stringify(envelope))
  }

  close(peerId = '') {
    this.send('room.leave', {}, peerId)
    this.clearPing()
    this.socket?.close()
    this.socket = null
  }

  private clearPing() {
    if (this.pingTimer !== null) {
      window.clearInterval(this.pingTimer)
      this.pingTimer = null
    }
  }
}