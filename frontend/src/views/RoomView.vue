<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Close } from '@element-plus/icons-vue'
import ChatPanel from '@/components/ChatPanel.vue'
import MemberList from '@/components/MemberList.vue'
import VideoGrid from '@/components/VideoGrid.vue'
import { joinRoom } from '@/api/room'
import { getErrorMessage } from '@/api/http'
import { LiveKitRoomConnection } from '@/livekit/client'
import { SignalingClient } from '@/signaling/client'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import { useConnectionStore, type RtcStatus, type SignalingStatus } from '@/stores/connection'
import { useRoomStore } from '@/stores/room'
import type { ChatMessage, PeerState, SignalingEnvelope } from '@/types'
import type { MediaTile } from '@/livekit/connection'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const roomStore = useRoomStore()
const chatStore = useChatStore()
const connectionStore = useConnectionStore()
const loading = ref(true)
const signaling = ref<SignalingClient | null>(null)
const livekit = ref<LiveKitRoomConnection | null>(null)
const mediaTiles = ref<MediaTile[]>([])
const roomId = computed(() => String(route.params.roomId ?? ''))
const appWsConnected = computed(() => connectionStore.signalingStatus === 'connected' && roomStore.joined)

onMounted(async () => {
  await enterRoom()
})

onBeforeUnmount(() => {
  shutdownConnections()
})

async function enterRoom() {
  if (!authStore.user) {
    await router.push('/login')
    return
  }
  loading.value = true
  connectionStore.clearError()
  connectionStore.setRtcStatus('idle')
  connectionStore.setSignalingStatus('idle')
  try {
    const connection = await joinRoom(roomId.value, authStore.user.userId, authStore.user.username)
    roomStore.setConnection(connection)
    chatStore.clear()
    mediaTiles.value = []

    livekit.value = new LiveKitRoomConnection({
      onStatusChange: (status) => connectionStore.setRtcStatus(status),
      onTilesChange: (tiles) => {
        mediaTiles.value = tiles
      },
      onError: (message) => {
        connectionStore.setError(message, { source: 'rtc' })
        ElMessage.error(message)
      },
    })
    await livekit.value.connect({ url: connection.livekitUrl, token: connection.livekitToken })

    signaling.value = new SignalingClient(
      connection.roomId,
      authStore.user,
      connection.livekitIdentity,
      handleMessage,
      (status, message) => {
        connectionStore.setSignalingStatus(status)
        if (message) {
          connectionStore.setError(message, { source: 'signaling' })
        }
      },
    )
    signaling.value.connect(connection.appWsUrl)
  } catch (error) {
    const message = getErrorMessage(error)
    connectionStore.setError(message, { source: 'general' })
    ElMessage.error(message)
  } finally {
    loading.value = false
  }
}

function handleMessage(message: SignalingEnvelope) {
  if (message.type === 'room.joined') {
    roomStore.setJoined(message.peerId ?? '')
    return
  }
  if (message.type === 'room.snapshot') {
    const payload = message.payload as { peers: PeerState[] }
    roomStore.setPeers(payload.peers ?? [])
    return
  }
  if (message.type === 'peer.joined') {
    roomStore.upsertPeer(message.payload as PeerState)
    return
  }
  if (message.type === 'peer.left') {
    roomStore.removePeer(message.peerId ?? '')
    return
  }
  if (message.type === 'chat.message') {
    const payload = message.payload as { messageId: string; content: string }
    chatStore.addMessage({
      messageId: payload.messageId,
      roomId: message.roomId,
      peerId: message.peerId ?? '',
      userId: message.userId,
      username: message.username,
      content: payload.content,
      sentAt: message.timestamp ?? Date.now(),
    } satisfies ChatMessage)
    return
  }
  if (message.type === 'error') {
    const payload = message.payload as { message?: string }
    ElMessage.error(payload.message ?? '信令错误')
  }
}

function sendChat(content: string) {
  signaling.value?.send('chat.send', { content }, roomStore.peerId)
}

async function leaveRoom() {
  shutdownConnections()
  roomStore.clear()
  chatStore.clear()
  await router.push('/')
}

function shutdownConnections() {
  signaling.value?.close(roomStore.peerId)
  signaling.value = null
  livekit.value?.disconnect()
  livekit.value = null
  mediaTiles.value = []
}

function rtcTagType(status: RtcStatus) {
  if (status === 'connected') {
    return 'success'
  }
  if (status === 'failed') {
    return 'danger'
  }
  if (status === 'reconnecting' || status === 'connecting') {
    return 'warning'
  }
  return 'info'
}

function signalingTagType(status: SignalingStatus) {
  if (status === 'connected') {
    return 'success'
  }
  if (status === 'error') {
    return 'danger'
  }
  if (status === 'connecting') {
    return 'warning'
  }
  return 'info'
}

function rtcStatusText(status: RtcStatus) {
  const labels: Record<RtcStatus, string> = {
    idle: '未连接',
    connecting: '连接中',
    connected: '已连接',
    reconnecting: '重连中',
    failed: '连接失败',
    closed: '已关闭',
  }
  return labels[status]
}

function signalingStatusText(status: SignalingStatus) {
  const labels: Record<SignalingStatus, string> = {
    idle: '未连接',
    connecting: '连接中',
    connected: '已连接',
    closed: '已关闭',
    error: '连接错误',
  }
  return labels[status]
}
</script>

<template>
  <main class="room-page">
    <header class="room-header">
      <div>
        <span>房间 {{ roomId }}</span>
        <h1>StreamForge 音视频房间</h1>
      </div>
      <div class="room-status">
        <el-tag :type="rtcTagType(connectionStore.rtcStatus)" effect="plain">
          LiveKit：{{ rtcStatusText(connectionStore.rtcStatus) }}
        </el-tag>
        <el-tag :type="signalingTagType(connectionStore.signalingStatus)" effect="plain">
          应用信令：{{ signalingStatusText(connectionStore.signalingStatus) }}
        </el-tag>
        <el-button :icon="Close" @click="leaveRoom">离开房间</el-button>
      </div>
    </header>

    <el-alert
      v-if="connectionStore.errorMessage"
      :title="connectionStore.errorMessage"
      type="error"
      show-icon
      :closable="false"
    />

    <section v-loading="loading" class="room-shell">
      <section class="stage">
        <VideoGrid :tiles="mediaTiles" :status="connectionStore.rtcStatus" />
      </section>

      <aside class="side-rail">
        <MemberList :peers="roomStore.peers" />
        <ChatPanel :disabled="!appWsConnected" @send="sendChat" />
      </aside>
    </section>
  </main>
</template>