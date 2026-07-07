import { describe, expect, it } from 'vitest'
import {
  compareMediaTiles,
  liveKitConnectionStateToStatus,
  roomEventToStatus,
  type MediaTile,
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

describe('compareMediaTiles', () => {
  it('keeps local video first, then remote videos, then remote audio-only tracks', () => {
    const tiles: MediaTile[] = [
      tile('remote-audio', false, 'audio'),
      tile('remote-video-b', false, 'video'),
      tile('local-video', true, 'video'),
      tile('remote-video-a', false, 'video'),
    ]

    expect([...tiles].sort(compareMediaTiles).map((item) => item.id)).toEqual([
      'local-video',
      'remote-video-a',
      'remote-video-b',
      'remote-audio',
    ])
  })
})

function tile(id: string, isLocal: boolean, kind: MediaTile['kind']): MediaTile {
  return {
    id,
    participantIdentity: id,
    participantName: id,
    isLocal,
    kind,
    source: 'camera',
    muted: false,
  }
}
