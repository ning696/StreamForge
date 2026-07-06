import axios from 'axios'

export const userHttp = axios.create({
  baseURL: import.meta.env.VITE_USER_API_BASE_URL ?? 'http://localhost:8081',
  timeout: 10000,
})

export const mediaHttp = axios.create({
  baseURL: import.meta.env.VITE_MEDIA_API_BASE_URL ?? 'http://localhost:8080',
  timeout: 10000,
})

export function getErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as { message?: string } | undefined
    return data?.message ?? error.message
  }
  return error instanceof Error ? error.message : '请求失败'
}
