import { defineStore } from 'pinia'
import type { PeerState, RoomConnectionInfo } from '@/types'

export const useRoomStore = defineStore('room', {
  state: () => ({
    roomId: '',
    livekitRoomName: '',
    livekitUrl: '',
    livekitToken: '',
    livekitIdentity: '',
    appWsUrl: '',
    peerId: '',
    joined: false,
    peers: [] as PeerState[],
  }),
  actions: {
    setConnection(info: RoomConnectionInfo) {
      this.roomId = info.roomId
      this.livekitRoomName = info.livekitRoomName
      this.livekitUrl = info.livekitUrl
      this.livekitToken = info.livekitToken
      this.livekitIdentity = info.livekitIdentity
      this.appWsUrl = info.appWsUrl
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
      this.livekitRoomName = ''
      this.livekitUrl = ''
      this.livekitToken = ''
      this.livekitIdentity = ''
      this.appWsUrl = ''
      this.peerId = ''
      this.joined = false
      this.peers = []
    },
  },
})