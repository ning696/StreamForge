export type LiveKitStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'failed' | 'closed'
export type MediaSource = 'camera' | 'microphone' | 'screen' | 'unknown'

export interface AttachableTrack {
  attach(element?: HTMLMediaElement): HTMLMediaElement
  detach(element?: HTMLMediaElement): HTMLMediaElement[] | HTMLMediaElement
}

export interface ParticipantTile {
  participantIdentity: string
  participantName: string
  isLocal: boolean
  joinOrder: number
  cameraTrack?: AttachableTrack
  microphoneTrack?: AttachableTrack
  screenShareTrack?: AttachableTrack
  cameraEnabled: boolean
  microphoneEnabled: boolean
  microphoneMuted: boolean
  screenSharing: boolean
  screenShareOrder?: number
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

export function compareParticipantTiles(left: ParticipantTile, right: ParticipantTile): number {
  if (left.isLocal !== right.isLocal) {
    return left.isLocal ? -1 : 1
  }
  return left.joinOrder - right.joinOrder || left.participantIdentity.localeCompare(right.participantIdentity)
}

export function getRemoteAudioParticipants(participants: ParticipantTile[]): ParticipantTile[] {
  return participants.filter((participant) => !participant.isLocal && participant.microphoneTrack)
}

export function getActiveScreenShare(participants: ParticipantTile[]): ParticipantTile | undefined {
  return participants
    .filter((participant) => participant.screenShareTrack && participant.screenSharing)
    .sort(
      (left, right) =>
        (right.screenShareOrder ?? -1) - (left.screenShareOrder ?? -1) ||
        compareParticipantTiles(left, right),
    )[0]
}
