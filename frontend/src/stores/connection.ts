import { defineStore } from 'pinia'

export type SignalingStatus = 'idle' | 'connecting' | 'connected' | 'closed' | 'error'
export type RtcStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'failed' | 'closed'

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
    setRtcStatus(status: RtcStatus) {
      this.rtcStatus = status
    },
    setError(message: string, options: { source?: 'signaling' | 'rtc' | 'general' } = {}) {
      if (options.source === 'signaling') {
        this.signalingStatus = 'error'
      }
      if (options.source === 'rtc' || options.source === 'general' || !options.source) {
        this.rtcStatus = 'failed'
      }
      this.errorMessage = message
    },
    clearError() {
      this.errorMessage = ''
    },
  },
})
