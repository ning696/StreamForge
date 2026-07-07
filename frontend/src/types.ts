export interface UserSession {
  userId: number
  account: string
  username: string
}

export interface RoomConnectionInfo {
  roomId: string
  livekitRoomName: string
  livekitUrl: string
  livekitToken: string
  livekitIdentity: string
  appWsUrl: string
}

export interface PeerState {
  peerId: string
  userId: number
  username: string
  livekitIdentity?: string
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