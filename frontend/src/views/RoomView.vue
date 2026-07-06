<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Close, Connection, VideoCamera } from '@element-plus/icons-vue'
import ChatPanel from '@/components/ChatPanel.vue'
import MemberList from '@/components/MemberList.vue'
import { joinRoom } from '@/api/room'
import { getErrorMessage } from '@/api/http'
import { SignalingClient } from '@/signaling/client'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import { useConnectionStore } from '@/stores/connection'
import { useRoomStore } from '@/stores/room'
import type { ChatMessage, PeerState, SignalingEnvelope } from '@/types'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const roomStore = useRoomStore()
const chatStore = useChatStore()
const connectionStore = useConnectionStore()
const loading = ref(true)
const signaling = ref<SignalingClient | null>(null)
const roomId = computed(() => String(route.params.roomId ?? ''))
const connected = computed(() => connectionStore.signalingStatus === 'connected' && roomStore.joined)

onMounted(async () => {
  await enterRoom()
})

onBeforeUnmount(() => {
  signaling.value?.close(roomStore.peerId)
})

async function enterRoom() {
  if (!authStore.user) {
    await router.push('/login')
    return
  }
  loading.value = true
  connectionStore.clearError()
  try {
    const routeInfo = await joinRoom(roomId.value, authStore.user.userId, authStore.user.username)
    roomStore.setRoute(routeInfo)
    chatStore.clear()
    signaling.value = new SignalingClient(
      routeInfo.roomId,
      authStore.user,
      handleMessage,
      (status, message) => {
        connectionStore.setSignalingStatus(status)
        if (message) {
          connectionStore.setError(message)
        }
      },
    )
    signaling.value.connect(routeInfo.wsUrl)
  } catch (error) {
    connectionStore.setError(getErrorMessage(error))
    ElMessage.error(getErrorMessage(error))
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
  signaling.value?.close(roomStore.peerId)
  roomStore.clear()
  chatStore.clear()
  await router.push('/')
}
</script>

<template>
  <main class="room-page">
    <header class="room-header">
      <div>
        <span>房间 {{ roomId }}</span>
        <h1>StreamForge M1 房间</h1>
      </div>
      <div class="room-status">
        <el-tag :type="connected ? 'success' : 'warning'" effect="plain">
          {{ connected ? '信令已连接' : connectionStore.signalingStatus }}
        </el-tag>
        <el-button :icon="Close" @click="leaveRoom">离开</el-button>
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
        <div class="stage-content">
          <el-icon><VideoCamera /></el-icon>
          <h2>音视频将在 Week 3-5 接入</h2>
          <p>当前阶段专注验证用户服务、Redis 房间路由、WebSocket 成员状态和聊天广播。</p>
          <div class="route-line">
            <el-icon><Connection /></el-icon>
            <span>媒体实例：{{ roomStore.mediaInstanceId || '等待路由' }}</span>
          </div>
        </div>
      </section>

      <aside class="side-rail">
        <MemberList :peers="roomStore.peers" />
        <ChatPanel :disabled="!connected" @send="sendChat" />
      </aside>
    </section>
  </main>
</template>
