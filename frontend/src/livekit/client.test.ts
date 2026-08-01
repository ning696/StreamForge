import { describe, expect, it } from 'vitest'
import { LiveKitRoomConnection, type LiveKitRoomLike } from './client'
import type { AttachableTrack, LiveKitStatus, ParticipantTile } from './connection'

describe('LiveKitRoomConnection', () => {
  it('groups camera and microphone tracks into one stable participant tile', async () => {
    const fakeRoom = new FakeRoom()
    const statuses: LiveKitStatus[] = []
    const snapshots: ParticipantTile[][] = []
    const connection = createConnection(fakeRoom, statuses, snapshots)

    await connection.connect({ url: 'ws://livekit.test', token: 'token' })

    expect(fakeRoom.connectedWith).toEqual(['ws://livekit.test', 'token'])
    expect(fakeRoom.localParticipant.cameraAndMicrophoneEnabled).toBe(true)
    expect(statuses).toEqual(['connecting', 'connected'])
    expect(snapshots.at(-1)?.map((participant) => participant.participantIdentity)).toEqual(['user-1'])

    const localCamera = mediaTrack('local-camera-track', 'video', 'camera')
    const localMicrophone = mediaTrack('local-microphone-track', 'audio', 'microphone')
    fakeRoom.emit('localTrackPublished', publication('local-camera', localCamera), fakeRoom.localParticipant)
    fakeRoom.emit(
      'localTrackPublished',
      publication('local-microphone', localMicrophone),
      fakeRoom.localParticipant,
    )

    const local = snapshots.at(-1)?.[0]
    expect(snapshots.at(-1)).toHaveLength(1)
    expect(local?.cameraTrack).toBe(localCamera)
    expect(local?.microphoneTrack).toBe(localMicrophone)
  })

  it('creates a camera-off placeholder on join and preserves ordering across media changes', async () => {
    const fakeRoom = new FakeRoom()
    const snapshots: ParticipantTile[][] = []
    const connection = createConnection(fakeRoom, [], snapshots)
    await connection.connect({ url: 'ws://livekit.test', token: 'token' })

    const bob = participant('user-2', 'Bob')
    const carol = participant('user-3', 'Carol')
    fakeRoom.emit('participantConnected', bob)
    fakeRoom.emit('participantConnected', carol)

    expect(snapshots.at(-1)?.map((item) => item.participantIdentity)).toEqual([
      'user-1',
      'user-2',
      'user-3',
    ])
    expect(snapshots.at(-1)?.[1]?.cameraEnabled).toBe(false)

    const bobCamera = mediaTrack('bob-camera-track', 'video', 'camera')
    const bobPublication = publication('bob-camera', bobCamera)
    fakeRoom.emit('trackSubscribed', bobCamera, bobPublication, bob)
    fakeRoom.emit('trackMuted', bobPublication, bob)
    fakeRoom.emit('trackUnmuted', bobPublication, bob)

    expect(snapshots.at(-1)?.map((item) => item.participantIdentity)).toEqual([
      'user-1',
      'user-2',
      'user-3',
    ])
    expect(snapshots.at(-1)?.[1]?.cameraEnabled).toBe(true)

    fakeRoom.emit('trackUnsubscribed', bobCamera, bobPublication, bob)
    expect(snapshots.at(-1)?.[1]?.cameraTrack).toBeUndefined()
    expect(snapshots.at(-1)?.[1]?.cameraEnabled).toBe(false)
    expect(snapshots.at(-1)).toHaveLength(3)
  })

  it('keeps remote audio on the participant and removes all media when the participant leaves', async () => {
    const fakeRoom = new FakeRoom()
    const snapshots: ParticipantTile[][] = []
    const connection = createConnection(fakeRoom, [], snapshots)
    await connection.connect({ url: 'ws://livekit.test', token: 'token' })

    const bob = participant('user-2', 'Bob')
    const audio = mediaTrack('bob-audio-track', 'audio', 'microphone')
    const audioPublication = publication('bob-audio', audio)
    fakeRoom.emit('participantConnected', bob)
    fakeRoom.emit('trackSubscribed', audio, audioPublication, bob)

    expect(snapshots.at(-1)).toHaveLength(2)
    expect(snapshots.at(-1)?.[1]?.microphoneTrack).toBe(audio)
    expect(snapshots.at(-1)?.[1]?.microphoneEnabled).toBe(true)

    fakeRoom.emit('participantDisconnected', bob)
    expect(snapshots.at(-1)?.map((item) => item.participantIdentity)).toEqual(['user-1'])

    connection.disconnect()
    expect(fakeRoom.disconnected).toBe(true)
    expect(snapshots.at(-1)).toEqual([])
  })

  it('keeps the room and local placeholder when media permission fails', async () => {
    const fakeRoom = new FakeRoom()
    fakeRoom.localParticipant.enableCameraAndMicrophone = async () => {
      throw new Error('Permission denied')
    }
    const statuses: LiveKitStatus[] = []
    const snapshots: ParticipantTile[][] = []
    const errors: string[] = []

    const connection = new LiveKitRoomConnection({
      createRoom: () => fakeRoom,
      onStatusChange: (status) => statuses.push(status),
      onParticipantsChange: (participants) => snapshots.push(participants),
      onError: (message) => errors.push(message),
    })

    await connection.connect({ url: 'ws://livekit.test', token: 'token' })

    expect(statuses).toEqual(['connecting', 'connected'])
    expect(errors).toEqual(['Permission denied'])
    expect(snapshots.at(-1)).toHaveLength(1)
    expect(snapshots.at(-1)?.[0]?.cameraEnabled).toBe(false)
  })

  it('tracks the latest screen share without creating an extra participant', async () => {
    const fakeRoom = new FakeRoom()
    const snapshots: ParticipantTile[][] = []
    const connection = createConnection(fakeRoom, [], snapshots)
    await connection.connect({ url: 'ws://livekit.test', token: 'token' })

    const bob = participant('user-2', 'Bob')
    const screen = mediaTrack('bob-screen-track', 'video', 'screen_share')
    const screenPublication = publication('bob-screen', screen)
    fakeRoom.emit('participantConnected', bob)
    fakeRoom.emit('trackSubscribed', screen, screenPublication, bob)

    expect(snapshots.at(-1)).toHaveLength(2)
    expect(snapshots.at(-1)?.[1]?.screenShareTrack).toBe(screen)
    expect(snapshots.at(-1)?.[1]?.screenSharing).toBe(true)

    fakeRoom.emit('trackUnsubscribed', screen, screenPublication, bob)
    expect(snapshots.at(-1)?.[1]?.screenShareTrack).toBeUndefined()
    expect(snapshots.at(-1)?.[1]?.screenSharing).toBe(false)
  })
})

function createConnection(
  fakeRoom: FakeRoom,
  statuses: LiveKitStatus[],
  snapshots: ParticipantTile[][],
) {
  return new LiveKitRoomConnection({
    createRoom: () => fakeRoom,
    onStatusChange: (status) => statuses.push(status),
    onParticipantsChange: (participants) => snapshots.push(participants),
  })
}

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

function publication(trackSid: string, track: MediaTrack) {
  return {
    trackSid,
    source: track.source,
    kind: track.kind,
    isMuted: false,
    track,
  }
}

type MediaTrack = AttachableTrack & { kind: string; sid: string; source: string }

function mediaTrack(sid: string, kind: string, source: string): MediaTrack {
  return {
    sid,
    kind,
    source,
    attach: () => document.createElement(kind === 'video' ? 'video' : 'audio'),
    detach: () => [],
  }
}
