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
    const route = await createRoom(authStore.user.userId, authStore.user.username)
    roomStore.setRoute(route)
    chatStore.clear()
    await router.push(`/rooms/${route.roomId}`)
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
    const route = await joinRoom(targetRoomId, authStore.user.userId, authStore.user.username)
    roomStore.setRoute(route)
    chatStore.clear()
    await router.push(`/rooms/${route.roomId}`)
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
        <h1>会议操作台</h1>
        <p>创建房间或输入房间号加入，先验证用户服务、Redis 房间路由和 WebSocket 聊天。</p>
      </div>
      <el-button :icon="SwitchButton" @click="logout">退出登录</el-button>
    </section>

    <section class="entry-grid">
      <article class="entry-panel">
        <span class="step-label">创建</span>
        <h2>开启一个新房间</h2>
        <p>媒体服务会生成 6 位房间号，并写入 Redis 房间级路由。</p>
        <el-button type="primary" :icon="Plus" :loading="creating" @click="create">创建房间</el-button>
      </article>

      <article class="entry-panel">
        <span class="step-label">加入</span>
        <h2>进入已有房间</h2>
        <p>加入前会读取 Redis 中已有的 roomId 到 mediaInstanceId 路由。</p>
        <div class="join-row">
          <el-input v-model.trim="roomId" placeholder="输入房间号" @keyup.enter="join" />
          <el-button type="primary" plain :icon="Right" :loading="joining" @click="join">加入</el-button>
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
