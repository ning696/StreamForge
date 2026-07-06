export interface UserSession {
  userId: number
  account: string
  username: string
}

export interface IceServer {
  urls: string[]
}

export interface RoomRoute {
  roomId: string
  mediaInstanceId: string
  wsUrl: string
  rtcConfig: {
    iceServers: IceServer[]
  }
}

export interface PeerState {
  peerId: string
  userId: number
  username: string
  audioEnabled: boolean
  videoEnabled: boolean
  screenSharing: boolean
}

export interface ChatMessage {
  messageId: string
  roomId: string
  peerId: string
  userId: number
  username: string
  content: string
  sentAt: number
}

export interface SignalingEnvelope<TPayload = unknown> {
  type: string
  requestId?: string
  roomId: string
  peerId?: string
  userId: number
  username: string
  timestamp?: number
  payload: TPayload
}
