export type LiveKitStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'failed' | 'closed'
export type MediaKind = 'audio' | 'video'
export type MediaSource = 'camera' | 'microphone' | 'screen' | 'unknown'

export interface AttachableTrack {
  attach(element?: HTMLMediaElement): HTMLMediaElement
  detach(element?: HTMLMediaElement): HTMLMediaElement[] | HTMLMediaElement
}

export interface MediaTile {
  id: string
  participantIdentity: string
  participantName: string
  isLocal: boolean
  kind: MediaKind
  source: MediaSource
  muted: boolean
  track?: AttachableTrack
}

export function liveKitConnectionStateToStatus(state: string): LiveKitStatus {
  switch (state) {
    case 'connecting':
      return 'connecting'
    case 'connected':
      return 'connected'
    case 'reconnecting':
      return 'reconnecting'
    case 'disconnected':
      return 'closed'
    default:
      return 'failed'
  }
}

export function roomEventToStatus(event: string): LiveKitStatus {
  switch (event) {
    case 'connected':
    case 'reconnected':
      return 'connected'
    case 'reconnecting':
      return 'reconnecting'
    case 'disconnected':
      return 'closed'
    default:
      return 'failed'
  }
}

export function compareMediaTiles(left: MediaTile, right: MediaTile): number {
  return tilePriority(left) - tilePriority(right) || left.id.localeCompare(right.id)
}

function tilePriority(tile: MediaTile) {
  if (tile.isLocal && tile.kind === 'video') {
    return 0
  }
  if (tile.kind === 'video') {
    return 1
  }
  if (tile.isLocal) {
    return 2
  }
  return 3
}