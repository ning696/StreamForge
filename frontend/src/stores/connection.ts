import { defineStore } from 'pinia'

export type SignalingStatus = 'idle' | 'connecting' | 'connected' | 'closed' | 'error'
export type RtcStatus = 'idle' | 'connecting' | 'connected' | 'failed' | 'closed'

export const useConnectionStore = defineStore('connection', {
  state: () => ({
    signalingStatus: 'idle' as SignalingStatus,
    rtcStatus: 'idle' as RtcStatus,
    errorMessage: '',
  }),
  actions: {
    setSignalingStatus(status: SignalingStatus) {
      this.signalingStatus = status
    },
    setError(message: string) {
      this.signalingStatus = 'error'
      this.errorMessage = message
    },
    clearError() {
      this.errorMessage = ''
    },
  },
})
