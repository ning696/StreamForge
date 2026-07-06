import { mediaHttp } from './http'
import type { RoomRoute } from '@/types'

export async function createRoom(userId: number, username: string): Promise<RoomRoute> {
  const { data } = await mediaHttp.post<RoomRoute>('/api/rooms', { userId, username })
  return data
}

export async function joinRoom(roomId: string, userId: number, username: string): Promise<RoomRoute> {
  const { data } = await mediaHttp.post<RoomRoute>(`/api/rooms/${roomId}/join`, { userId, username })
  return data
}
