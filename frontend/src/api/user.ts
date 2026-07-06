import { userHttp } from './http'
import type { UserSession } from '@/types'

export interface RegisterPayload {
  account: string
  password: string
  username: string
}

export interface LoginPayload {
  account: string
  password: string
}

export async function register(payload: RegisterPayload): Promise<UserSession> {
  const { data } = await userHttp.post<UserSession>('/api/users/register', payload)
  return data
}

export async function login(payload: LoginPayload): Promise<UserSession> {
  const { data } = await userHttp.post<UserSession>('/api/users/login', payload)
  return data
}

export async function getUser(userId: number): Promise<UserSession> {
  const { data } = await userHttp.get<UserSession>(`/api/users/${userId}`)
  return data
}
