import { describe, expect, it } from 'vitest'
import {
  compareParticipantTiles,
  getActiveScreenShare,
  getRemoteAudioParticipants,
  liveKitConnectionStateToStatus,
  roomEventToStatus,
  type ParticipantTile,
} from './connection'

describe('liveKitConnectionStateToStatus', () => {
  it('maps LiveKit connection states to reader-facing room status', () => {
    expect(liveKitConnectionStateToStatus('connected')).toBe('connected')
    expect(liveKitConnectionStateToStatus('connecting')).toBe('connecting')
    expect(liveKitConnectionStateToStatus('reconnecting')).toBe('reconnecting')
    expect(liveKitConnectionStateToStatus('disconnected')).toBe('closed')
    expect(liveKitConnectionStateToStatus('unknown')).toBe('failed')
  })
})

describe('roomEventToStatus', () => {
  it('maps high-level room lifecycle events to room status', () => {
    expect(roomEventToStatus('connected')).toBe('connected')
    expect(roomEventToStatus('reconnecting')).toBe('reconnecting')
    expect(roomEventToStatus('reconnected')).toBe('connected')
    expect(roomEventToStatus('disconnected')).toBe('closed')
  })
})

describe('participant view helpers', () => {
  it('keeps the local participant first and preserves remote join order', () => {
    const participants = [
      participant('remote-2', false, 2),
      participant('local', true, 0),
      participant('remote-1', false, 1),
    ]

    expect([...participants].sort(compareParticipantTiles).map((item) => item.participantIdentity)).toEqual([
      'local',
      'remote-1',
      'remote-2',
    ])
  })

  it('selects only remote microphone tracks for hidden playback', () => {
    const local = participant('local', true, 0)
    const remoteWithAudio = participant('remote-audio', false, 1)
    const remoteWithoutAudio = participant('remote-silent', false, 2)
    local.microphoneTrack = track()
    remoteWithAudio.microphoneTrack = track()

    expect(getRemoteAudioParticipants([local, remoteWithAudio, remoteWithoutAudio])).toEqual([
      remoteWithAudio,
    ])
  })

  it('selects the most recently started screen share', () => {
    const first = participant('first', false, 1)
    const latest = participant('latest', false, 2)
    first.screenShareTrack = track()
    first.screenSharing = true
    first.screenShareOrder = 3
    latest.screenShareTrack = track()
    latest.screenSharing = true
    latest.screenShareOrder = 4

    expect(getActiveScreenShare([first, latest])).toBe(latest)
  })
})

function participant(identity: string, isLocal: boolean, joinOrder: number): ParticipantTile {
  return {
    participantIdentity: identity,
    participantName: identity,
    isLocal,
    joinOrder,
    cameraEnabled: false,
    microphoneEnabled: false,
    microphoneMuted: false,
    screenSharing: false,
  }
}

function track() {
  return {
    attach: () => document.createElement('video'),
    detach: () => [],
  }
}
