import { defineStore } from 'pinia'
import type { ChatMessage } from '@/types'

export const useChatStore = defineStore('chat', {
  state: () => ({
    messages: [] as ChatMessage[],
    unreadCount: 0,
  }),
  actions: {
    addMessage(message: ChatMessage) {
      this.messages.push(message)
    },
    clear() {
      this.messages = []
      this.unreadCount = 0
    },
  },
})
