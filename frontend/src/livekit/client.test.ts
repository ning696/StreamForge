import { describe, expect, it } from 'vitest'
import { LiveKitRoomConnection, type LiveKitRoomLike } from './client'
import type { AttachableTrack, LiveKitStatus, MediaTile } from './connection'

describe('LiveKitRoomConnection', () => {
  it('connects, enables local media, emits media tiles, and cleans up on disconnect', async () => {
    const fakeRoom = new FakeRoom()
    const statuses: LiveKitStatus[] = []
    const tileSnapshots: MediaTile[][] = []

    const connection = new LiveKitRoomConnection({
      createRoom: () => fakeRoom,
      onStatusChange: (status) => statuses.push(status),
      onTilesChange: (tiles) => tileSnapshots.push(tiles),
    })

    await connection.connect({ url: 'ws://livekit.test', token: 'token' })

    expect(fakeRoom.connectedWith).toEqual(['ws://livekit.test', 'token'])
    expect(fakeRoom.localParticipant.cameraAndMicrophoneEnabled).toBe(true)
    expect(statuses).toEqual(['connecting', 'connected'])

    fakeRoom.emit('localTrackPublished', publication('local-camera', localVideoTrack()), fakeRoom.localParticipant)
    fakeRoom.emit(
      'trackSubscribed',
      remoteVideoTrack(),
      publication('remote-camera', remoteVideoTrack()),
      participant('user-2', 'Bob'),
    )

    expect(tileSnapshots.at(-1)?.map((tile) => tile.id)).toEqual([
      'local:user-1:local-camera',
      'remote:user-2:remote-camera',
    ])

    fakeRoom.emit(
      'trackUnsubscribed',
      remoteVideoTrack(),
      publication('remote-camera', remoteVideoTrack()),
      participant('user-2', 'Bob'),
    )

    expect(tileSnapshots.at(-1)?.map((tile) => tile.id)).toEqual(['local:user-1:local-camera'])

    connection.disconnect()

    expect(fakeRoom.disconnected).toBe(true)
    expect(tileSnapshots.at(-1)).toEqual([])
    expect(statuses.at(-1)).toBe('closed')
  })

  it('keeps the room connected when local media permission fails', async () => {
    const fakeRoom = new FakeRoom()
    fakeRoom.localParticipant.enableCameraAndMicrophone = async () => {
      throw new Error('Permission denied')
    }
    const statuses: LiveKitStatus[] = []
    const errors: string[] = []

    const connection = new LiveKitRoomConnection({
      createRoom: () => fakeRoom,
      onStatusChange: (status) => statuses.push(status),
      onError: (message) => errors.push(message),
    })

    await connection.connect({ url: 'ws://livekit.test', token: 'token' })

    expect(statuses).toEqual(['connecting', 'connected'])
    expect(errors).toEqual(['Permission denied'])
  })
})

class FakeRoom implements LiveKitRoomLike {
  readonly localParticipant = {
    identity: 'user-1',
    name: 'Alice',
    cameraAndMicrophoneEnabled: false,
    enableCameraAndMicrophone: async () => {
      this.localParticipant.cameraAndMicrophoneEnabled = true
    },
  }

  readonly remoteParticipants = new Map()
  connectedWith: string[] = []
  disconnected = false
  private handlers = new Map<string, Function[]>()

  on(event: string, handler: Function): this {
    this.handlers.set(event, [...(this.handlers.get(event) ?? []), handler])
    return this
  }

  off(event: string, handler: Function): this {
    this.handlers.set(
      event,
      (this.handlers.get(event) ?? []).filter((item) => item !== handler),
    )
    return this
  }

  async connect(url: string, token: string): Promise<void> {
    this.connectedWith = [url, token]
  }

  disconnect(): void {
    this.disconnected = true
  }

  emit(event: string, ...args: unknown[]) {
    for (const handler of this.handlers.get(event) ?? []) {
      handler(...args)
    }
  }
}

function participant(identity: string, name: string) {
  return { identity, name }
}

function publication(trackSid: string, track: AttachableTrack) {
  return {
    trackSid,
    source: trackSid.includes('camera') ? 'camera' : 'microphone',
    isMuted: false,
    track,
  }
}

function localVideoTrack(): AttachableTrack & { kind: string; sid: string; source: string } {
  return {
    sid: 'local-track',
    kind: 'video',
    source: 'camera',
    attach: () => document.createElement('video'),
    detach: () => [],
  }
}

function remoteVideoTrack(): AttachableTrack & { kind: string; sid: string; source: string } {
  return {
    sid: 'remote-track',
    kind: 'video',
    source: 'camera',
    attach: () => document.createElement('video'),
    detach: () => [],
  }
}
