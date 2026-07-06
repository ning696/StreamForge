import { defineStore } from 'pinia'
import type { PeerState, RoomRoute } from '@/types'

export const useRoomStore = defineStore('room', {
  state: () => ({
    roomId: '',
    mediaInstanceId: '',
    wsUrl: '',
    peerId: '',
    joined: false,
    peers: [] as PeerState[],
  }),
  actions: {
    setRoute(route: RoomRoute) {
      this.roomId = route.roomId
      this.mediaInstanceId = route.mediaInstanceId
      this.wsUrl = route.wsUrl
    },
    setJoined(peerId: string) {
      this.peerId = peerId
      this.joined = true
    },
    setPeers(peers: PeerState[]) {
      this.peers = peers
    },
    upsertPeer(peer: PeerState) {
      const index = this.peers.findIndex((item) => item.peerId === peer.peerId)
      if (index >= 0) {
        this.peers[index] = peer
      } else {
        this.peers.push(peer)
      }
    },
    removePeer(peerId: string) {
      this.peers = this.peers.filter((peer) => peer.peerId !== peerId)
    },
    clear() {
      this.roomId = ''
      this.mediaInstanceId = ''
      this.wsUrl = ''
      this.peerId = ''
      this.joined = false
      this.peers = []
    },
  },
})
