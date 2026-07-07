<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, Right, SwitchButton } from '@element-plus/icons-vue'
import { createRoom, joinRoom } from '@/api/room'
import { getErrorMessage } from '@/api/http'
import { useAuthStore } from '@/stores/auth'
import { useRoomStore } from '@/stores/room'
import { useChatStore } from '@/stores/chat'

const router = useRouter()
const authStore = useAuthStore()
const roomStore = useRoomStore()
const chatStore = useChatStore()
const roomId = ref('')
const creating = ref(false)
const joining = ref(false)

async function create() {
  if (!authStore.user) {
    return
  }
  creating.value = true
  try {
    const connection = await createRoom(authStore.user.userId, authStore.user.username)
    roomStore.setConnection(connection)
    chatStore.clear()
    await router.push(`/rooms/${connection.roomId}`)
  } catch (error) {
    ElMessage.error(getErrorMessage(error))
  } finally {
    creating.value = false
  }
}

async function join() {
  if (!authStore.user) {
    return
  }
  const targetRoomId = roomId.value.trim()
  if (!targetRoomId) {
    ElMessage.warning('请输入房间号')
    return
  }
  joining.value = true
  try {
    const connection = await joinRoom(targetRoomId, authStore.user.userId, authStore.user.username)
    roomStore.setConnection(connection)
    chatStore.clear()
    await router.push(`/rooms/${connection.roomId}`)
  } catch (error) {
    ElMessage.error(getErrorMessage(error))
  } finally {
    joining.value = false
  }
}

function logout() {
  authStore.logout()
  roomStore.clear()
  chatStore.clear()
  router.push('/login')
}
</script>

<template>
  <main class="home-page">
    <section class="home-hero">
      <div>
        <h1>会议控制台</h1>
        <p>创建业务房间或加入已有房间，由房间服务签发 LiveKit 入会凭证，并保留应用聊天通道。</p>
      </div>
      <el-button :icon="SwitchButton" @click="logout">退出登录</el-button>
    </section>

    <section class="entry-grid">
      <article class="entry-panel">
        <span class="step-label">创建</span>
        <h2>发起新房间</h2>
        <p>房间服务会创建 StreamForge 业务房间，并映射到稳定的 LiveKit 房间名。</p>
        <el-button type="primary" :icon="Plus" :loading="creating" @click="create">创建房间</el-button>
      </article>

      <article class="entry-panel">
        <span class="step-label">加入</span>
        <h2>加入已有房间</h2>
        <p>加入成功后会获得新的 LiveKit Token，以及可选的应用 WebSocket 聊天地址。</p>
        <div class="join-row">
          <el-input v-model.trim="roomId" placeholder="请输入房间号" @keyup.enter="join" />
          <el-button type="primary" plain :icon="Right" :loading="joining" @click="join">加入房间</el-button>
        </div>
      </article>
    </section>

    <section class="identity-strip">
      <span>当前用户</span>
      <strong>{{ authStore.user?.username }}</strong>
      <span>{{ authStore.user?.account }}</span>
      <span>ID {{ authStore.user?.userId }}</span>
    </section>
  </main>
</template>