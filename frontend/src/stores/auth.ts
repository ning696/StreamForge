import { defineStore } from 'pinia'
import type { UserSession } from '@/types'

const STORAGE_KEY = 'streamforge.user'

function loadUser(): UserSession | null {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as UserSession
  } catch {
    localStorage.removeItem(STORAGE_KEY)
    return null
  }
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: loadUser() as UserSession | null,
  }),
  getters: {
    isLoggedIn: (state) => state.user !== null,
  },
  actions: {
    setUser(user: UserSession) {
      this.user = user
      localStorage.setItem(STORAGE_KEY, JSON.stringify(user))
    },
    logout() {
      this.user = null
      localStorage.removeItem(STORAGE_KEY)
    },
  },
})
