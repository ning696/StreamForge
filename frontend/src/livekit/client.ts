import { Room, RoomEvent } from 'livekit-client'
import {
  compareMediaTiles,
  liveKitConnectionStateToStatus,
  roomEventToStatus,
  type AttachableTrack,
  type LiveKitStatus,
  type MediaKind,
  type MediaSource,
  type MediaTile,
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
  onTilesChange?: (tiles: MediaTile[]) => void
  onError?: (message: string) => void
}

type RoomHandler = (...args: any[]) => void

export class LiveKitRoomConnection {
  private readonly createRoom: () => LiveKitRoomLike
  private readonly onStatusChange?: (status: LiveKitStatus) => void
  private readonly onTilesChange?: (tiles: MediaTile[]) => void
  private readonly onError?: (message: string) => void
  private room: LiveKitRoomLike | null = null
  private readonly tiles = new Map<string, MediaTile>()
  private readonly handlers: Array<[string, RoomHandler]> = []

  constructor(options: LiveKitRoomConnectionOptions = {}) {
    this.createRoom = options.createRoom ?? (() => new Room() as unknown as LiveKitRoomLike)
    this.onStatusChange = options.onStatusChange
    this.onTilesChange = options.onTilesChange
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
    await this.enableLocalMedia(room)
    this.collectExistingTiles(room)
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
    this.tiles.clear()
    this.emitTiles()
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
    this.on(room, RoomEvent.TrackSubscribed, (track, publication, participant) => {
      this.upsertTile(track, publication, participant, false)
    })
    this.on(room, RoomEvent.TrackUnsubscribed, (track, publication, participant) => {
      this.removeTile(track, publication, participant, false)
    })
    this.on(room, RoomEvent.LocalTrackPublished, (publication, participant) => {
      this.upsertTile(publication?.track, publication, participant ?? room.localParticipant, true)
    })
    this.on(room, RoomEvent.LocalTrackUnpublished, (publication, participant) => {
      this.removeTile(publication?.track, publication, participant ?? room.localParticipant, true)
    })
    this.on(room, RoomEvent.ParticipantDisconnected, (participant) => {
      this.removeParticipantTiles(participant?.identity)
    })
    this.on(room, RoomEvent.ConnectionStateChanged, (state) => {
      this.setStatus(liveKitConnectionStateToStatus(String(state)))
    })
    this.on(room, RoomEvent.Reconnecting, () => this.setStatus(roomEventToStatus('reconnecting')))
    this.on(room, RoomEvent.Reconnected, () => this.setStatus(roomEventToStatus('reconnected')))
    this.on(room, RoomEvent.Disconnected, () => {
      this.tiles.clear()
      this.emitTiles()
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

  private collectExistingTiles(room: LiveKitRoomLike) {
    for (const publication of toPublications(room.localParticipant.trackPublications)) {
      this.upsertTile(publication.track, publication, room.localParticipant, true)
    }
    for (const participant of room.remoteParticipants?.values() ?? []) {
      for (const publication of toPublications(participant.trackPublications)) {
        this.upsertTile(publication.track, publication, participant, false)
      }
    }
  }

  private upsertTile(
    track: LiveKitPublicationLike['track'] | undefined,
    publication: LiveKitPublicationLike | undefined,
    participant: LiveKitParticipantLike | undefined,
    isLocal: boolean,
  ) {
    if (!track || !publication || !participant) {
      return
    }
    const kind = normalizeKind(track.kind ?? publication.kind)
    if (!kind) {
      return
    }
    const id = createTileId(participant, publication, track, isLocal)
    this.tiles.set(id, {
      id,
      participantIdentity: participant.identity,
      participantName: participant.name || participant.identity,
      isLocal,
      kind,
      source: normalizeSource(track.source ?? publication.source),
      muted: Boolean(publication.isMuted),
      track,
    })
    this.emitTiles()
  }

  private removeTile(
    track: LiveKitPublicationLike['track'] | undefined,
    publication: LiveKitPublicationLike | undefined,
    participant: LiveKitParticipantLike | undefined,
    isLocal: boolean,
  ) {
    if (!publication || !participant) {
      return
    }
    this.tiles.delete(createTileId(participant, publication, track, isLocal))
    this.emitTiles()
  }

  private removeParticipantTiles(identity?: string) {
    if (!identity) {
      return
    }
    for (const [id, tile] of this.tiles) {
      if (tile.participantIdentity === identity) {
        this.tiles.delete(id)
      }
    }
    this.emitTiles()
  }

  private setMuted(
    publication: LiveKitPublicationLike | undefined,
    participant: LiveKitParticipantLike | undefined,
    muted: boolean,
  ) {
    if (!publication || !participant) {
      return
    }
    for (const [id, tile] of this.tiles) {
      const publicationId = publication.trackSid ?? publication.sid ?? publication.track?.sid
      if (tile.participantIdentity === participant.identity && id.includes(`:${publicationId}`)) {
        this.tiles.set(id, { ...tile, muted })
      }
    }
    this.emitTiles()
  }

  private emitTiles() {
    this.onTilesChange?.([...this.tiles.values()].sort(compareMediaTiles))
  }

  private setStatus(status: LiveKitStatus) {
    this.onStatusChange?.(status)
  }
}

function toPublications(publications?: Map<string, LiveKitPublicationLike>): LiveKitPublicationLike[] {
  return [...(publications?.values() ?? [])].filter((publication) => publication.track)
}

function createTileId(
  participant: LiveKitParticipantLike,
  publication: LiveKitPublicationLike,
  track: LiveKitPublicationLike['track'] | undefined,
  isLocal: boolean,
) {
  const publicationId = publication.trackSid ?? publication.sid ?? track?.sid ?? 'track'
  return `${isLocal ? 'local' : 'remote'}:${participant.identity}:${publicationId}`
}

function normalizeKind(kind?: string): MediaKind | null {
  if (kind === 'video' || kind === 'audio') {
    return kind
  }
  return null
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
