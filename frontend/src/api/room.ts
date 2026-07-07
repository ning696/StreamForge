import { mediaHttp } from './http'
import type { RoomConnectionInfo } from '@/types'

export async function createRoom(userId: number, username: string): Promise<RoomConnectionInfo> {
  const { data } = await mediaHttp.post<RoomConnectionInfo>('/api/rooms', { userId, username })
  return data
}

export async function joinRoom(roomId: string, userId: number, username: string): Promise<RoomConnectionInfo> {
  const { data } = await mediaHttp.post<RoomConnectionInfo>(`/api/rooms/${roomId}/join`, { userId, username })
  return data
}