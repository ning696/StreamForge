import { Room, RoomEvent } from 'livekit-client'
import {
  compareParticipantTiles,
  liveKitConnectionStateToStatus,
  roomEventToStatus,
  type AttachableTrack,
  type LiveKitStatus,
  type MediaSource,
  type ParticipantTile,
} from './connection'

export interface LiveKitConnectOptions {
  url: string
  token: string
}

export interface LiveKitRoomLike {
  localParticipant: LiveKitParticipantLike
  remoteParticipants?: Map<string, LiveKitParticipantLike>
  on(event: string, handler: (...args: any[]) => void): this
  off(event: string, handler: (...args: any[]) => void): this
  connect(url: string, token: string): Promise<void>
  disconnect(): void
}

export interface LiveKitParticipantLike {
  identity: string
  name?: string
  trackPublications?: Map<string, LiveKitPublicationLike>
  enableCameraAndMicrophone?: () => Promise<void>
}

export interface LiveKitPublicationLike {
  trackSid?: string
  sid?: string
  source?: string
  kind?: string
  isMuted?: boolean
  track?: AttachableTrack & {
    sid?: string
    kind?: string
    source?: string
  }
}

export interface LiveKitRoomConnectionOptions {
  createRoom?: () => LiveKitRoomLike
  onStatusChange?: (status: LiveKitStatus) => void
  onParticipantsChange?: (participants: ParticipantTile[]) => void
  onError?: (message: string) => void
}

type RoomHandler = (...args: any[]) => void

export class LiveKitRoomConnection {
  private readonly createRoom: () => LiveKitRoomLike
  private readonly onStatusChange?: (status: LiveKitStatus) => void
  private readonly onParticipantsChange?: (participants: ParticipantTile[]) => void
  private readonly onError?: (message: string) => void
  private room: LiveKitRoomLike | null = null
  private readonly participants = new Map<string, ParticipantTile>()
  private readonly handlers: Array<[string, RoomHandler]> = []
  private nextJoinOrder = 0
  private nextScreenShareOrder = 0

  constructor(options: LiveKitRoomConnectionOptions = {}) {
    this.createRoom = options.createRoom ?? (() => new Room() as unknown as LiveKitRoomLike)
    this.onStatusChange = options.onStatusChange
    this.onParticipantsChange = options.onParticipantsChange
    this.onError = options.onError
  }

  async connect(options: LiveKitConnectOptions): Promise<void> {
    this.disconnect(false)
    const room = this.createRoom()
    this.room = room
    this.bindRoomEvents(room)
    this.setStatus('connecting')

    try {
      await room.connect(options.url, options.token)
    } catch (error) {
      const message = getErrorMessage(error)
      this.setStatus('failed')
      this.onError?.(message)
      throw error
    }

    this.setStatus('connected')
    this.collectExistingParticipants(room)
    await this.enableLocalMedia(room)
    this.collectExistingTracks(room)
  }

  disconnect(emitClosed = true): void {
    if (this.room) {
      for (const [event, handler] of this.handlers) {
        this.room.off(event, handler)
      }
      this.handlers.length = 0
      this.room.disconnect()
      this.room = null
    }
    this.participants.clear()
    this.nextJoinOrder = 0
    this.nextScreenShareOrder = 0
    this.emitParticipants()
    if (emitClosed) {
      this.setStatus('closed')
    }
  }

  private async enableLocalMedia(room: LiveKitRoomLike) {
    try {
      await room.localParticipant.enableCameraAndMicrophone?.()
    } catch (error) {
      this.onError?.(getErrorMessage(error) || '媒体设备访问失败')
    }
  }

  private bindRoomEvents(room: LiveKitRoomLike) {
    this.on(room, RoomEvent.ParticipantConnected, (participant) => {
      this.upsertParticipant(participant, false)
    })
    this.on(room, RoomEvent.TrackSubscribed, (track, publication, participant) => {
      this.upsertTrack(track, publication, participant, false)
    })
    this.on(room, RoomEvent.TrackUnsubscribed, (track, publication, participant) => {
      this.removeTrack(track, publication, participant)
    })
    this.on(room, RoomEvent.LocalTrackPublished, (publication, participant) => {
      this.upsertTrack(publication?.track, publication, participant ?? room.localParticipant, true)
    })
    this.on(room, RoomEvent.LocalTrackUnpublished, (publication, participant) => {
      this.removeTrack(publication?.track, publication, participant ?? room.localParticipant)
    })
    this.on(room, RoomEvent.ParticipantDisconnected, (participant) => {
      this.removeParticipant(participant?.identity)
    })
    this.on(room, RoomEvent.ConnectionStateChanged, (state) => {
      this.setStatus(liveKitConnectionStateToStatus(String(state)))
    })
    this.on(room, RoomEvent.Reconnecting, () => this.setStatus(roomEventToStatus('reconnecting')))
    this.on(room, RoomEvent.Reconnected, () => this.setStatus(roomEventToStatus('reconnected')))
    this.on(room, RoomEvent.Disconnected, () => {
      this.participants.clear()
      this.emitParticipants()
      this.setStatus(roomEventToStatus('disconnected'))
    })
    this.on(room, RoomEvent.MediaDevicesError, (error) => {
      this.onError?.(getErrorMessage(error) || '媒体设备访问失败')
    })
    this.on(room, RoomEvent.TrackMuted, (publication, participant) => {
      this.setMuted(publication, participant, true)
    })
    this.on(room, RoomEvent.TrackUnmuted, (publication, participant) => {
      this.setMuted(publication, participant, false)
    })
  }

  private on(room: LiveKitRoomLike, event: string, handler: RoomHandler) {
    room.on(event, handler)
    this.handlers.push([event, handler])
  }

  private collectExistingParticipants(room: LiveKitRoomLike) {
    this.upsertParticipant(room.localParticipant, true)
    for (const participant of room.remoteParticipants?.values() ?? []) {
      this.upsertParticipant(participant, false)
    }
  }

  private collectExistingTracks(room: LiveKitRoomLike) {
    for (const publication of toPublications(room.localParticipant.trackPublications)) {
      this.upsertTrack(publication.track, publication, room.localParticipant, true)
    }
    for (const participant of room.remoteParticipants?.values() ?? []) {
      for (const publication of toPublications(participant.trackPublications)) {
        this.upsertTrack(publication.track, publication, participant, false)
      }
    }
  }

  private upsertParticipant(participant: LiveKitParticipantLike | undefined, isLocal: boolean) {
    if (!participant?.identity) {
      return
    }
    const existing = this.participants.get(participant.identity)
    if (existing) {
      this.participants.set(participant.identity, {
        ...existing,
        participantName: participant.name || participant.identity,
        isLocal: existing.isLocal || isLocal,
      })
    } else {
      this.participants.set(participant.identity, {
        participantIdentity: participant.identity,
        participantName: participant.name || participant.identity,
        isLocal,
        joinOrder: this.nextJoinOrder++,
        cameraEnabled: false,
        microphoneEnabled: false,
        microphoneMuted: false,
        screenSharing: false,
      })
    }
    this.emitParticipants()
  }

  private upsertTrack(
    track: LiveKitPublicationLike['track'] | undefined,
    publication: LiveKitPublicationLike | undefined,
    participant: LiveKitParticipantLike | undefined,
    isLocal: boolean,
  ) {
    if (!track || !publication || !participant) {
      return
    }
    this.upsertParticipant(participant, isLocal)
    const existing = this.participants.get(participant.identity)
    if (!existing) {
      return
    }

    const source = resolveSource(track, publication)
    const muted = Boolean(publication.isMuted)
    if (source === 'camera') {
      this.participants.set(participant.identity, {
        ...existing,
        cameraTrack: track,
        cameraEnabled: !muted,
      })
    } else if (source === 'microphone') {
      this.participants.set(participant.identity, {
        ...existing,
        microphoneTrack: track,
        microphoneEnabled: !muted,
        microphoneMuted: muted,
      })
    } else if (source === 'screen') {
      const isNewShare = existing.screenShareTrack !== track
      this.participants.set(participant.identity, {
        ...existing,
        screenShareTrack: track,
        screenSharing: !muted,
        screenShareOrder: isNewShare ? this.nextScreenShareOrder++ : existing.screenShareOrder,
      })
    }
    this.emitParticipants()
  }

  private removeTrack(
    track: LiveKitPublicationLike['track'] | undefined,
    publication: LiveKitPublicationLike | undefined,
    participant: LiveKitParticipantLike | undefined,
  ) {
    if (!publication || !participant) {
      return
    }
    const existing = this.participants.get(participant.identity)
    if (!existing) {
      return
    }

    const source = resolveSource(track, publication)
    if (source === 'camera') {
      this.participants.set(participant.identity, {
        ...existing,
        cameraTrack: undefined,
        cameraEnabled: false,
      })
    } else if (source === 'microphone') {
      this.participants.set(participant.identity, {
        ...existing,
        microphoneTrack: undefined,
        microphoneEnabled: false,
        microphoneMuted: true,
      })
    } else if (source === 'screen') {
      this.participants.set(participant.identity, {
        ...existing,
        screenShareTrack: undefined,
        screenSharing: false,
        screenShareOrder: undefined,
      })
    }
    this.emitParticipants()
  }

  private removeParticipant(identity?: string) {
    if (!identity) {
      return
    }
    this.participants.delete(identity)
    this.emitParticipants()
  }

  private setMuted(
    publication: LiveKitPublicationLike | undefined,
    participant: LiveKitParticipantLike | undefined,
    muted: boolean,
  ) {
    if (!publication || !participant) {
      return
    }
    const existing = this.participants.get(participant.identity)
    if (!existing) {
      return
    }

    const source = resolveSource(publication.track, publication)
    if (source === 'camera') {
      this.participants.set(participant.identity, { ...existing, cameraEnabled: !muted })
    } else if (source === 'microphone') {
      this.participants.set(participant.identity, {
        ...existing,
        microphoneEnabled: !muted,
        microphoneMuted: muted,
      })
    } else if (source === 'screen') {
      this.participants.set(participant.identity, { ...existing, screenSharing: !muted })
    }
    this.emitParticipants()
  }

  private emitParticipants() {
    this.onParticipantsChange?.([...this.participants.values()].sort(compareParticipantTiles))
  }

  private setStatus(status: LiveKitStatus) {
    this.onStatusChange?.(status)
  }
}

function toPublications(publications?: Map<string, LiveKitPublicationLike>): LiveKitPublicationLike[] {
  return [...(publications?.values() ?? [])].filter((publication) => publication.track)
}

function resolveSource(
  track: LiveKitPublicationLike['track'] | undefined,
  publication: LiveKitPublicationLike,
): MediaSource {
  const source = normalizeSource(track?.source ?? publication.source)
  if (source !== 'unknown') {
    return source
  }
  const kind = track?.kind ?? publication.kind
  if (kind === 'video') {
    return 'camera'
  }
  if (kind === 'audio') {
    return 'microphone'
  }
  return 'unknown'
}

function normalizeSource(source?: string): MediaSource {
  if (source === 'camera') {
    return 'camera'
  }
  if (source === 'microphone') {
    return 'microphone'
  }
  if (source === 'screen' || source === 'screen_share' || source === 'screenShare') {
    return 'screen'
  }
  return 'unknown'
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  if (typeof error === 'string') {
    return error
  }
  return 'LiveKit 连接失败'
}
