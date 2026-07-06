<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useChatStore } from '@/stores/chat'

const props = defineProps<{
  disabled: boolean
}>()

const emit = defineEmits<{
  send: [content: string]
}>()

const chatStore = useChatStore()
const draft = ref('')
const canSend = computed(() => draft.value.trim().length > 0 && !props.disabled)

function sendMessage() {
  const content = draft.value.trim()
  if (!content) {
    return
  }
  if (content.length > 1000) {
    ElMessage.error('聊天消息不能超过 1000 个字符')
    return
  }
  emit('send', content)
  draft.value = ''
}

function formatTime(timestamp: number) {
  return new Date(timestamp || Date.now()).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <section class="chat-panel">
    <header class="panel-title">房间聊天</header>
    <div class="messages">
      <p v-if="chatStore.messages.length === 0" class="empty">还没有消息，发送第一句问候。</p>
      <article v-for="message in chatStore.messages" :key="message.messageId" class="message">
        <div class="message-meta">
          <strong>{{ message.username }}</strong>
          <span>{{ formatTime(message.sentAt) }}</span>
        </div>
        <p>{{ message.content }}</p>
      </article>
    </div>
    <div class="composer">
      <el-input
        v-model="draft"
        :disabled="disabled"
        :rows="3"
        maxlength="1000"
        show-word-limit
        type="textarea"
        placeholder="输入消息"
        @keydown.enter.exact.prevent="sendMessage"
      />
      <el-button type="primary" :disabled="!canSend" @click="sendMessage">发送</el-button>
    </div>
  </section>
</template>

<style scoped>
.chat-panel {
  display: grid;
  min-height: 0;
  grid-template-rows: auto 1fr auto;
  gap: 14px;
}

.panel-title {
  font-size: 15px;
  font-weight: 700;
  color: #172033;
}

.messages {
  display: grid;
  align-content: start;
  gap: 10px;
  min-height: 180px;
  overflow: auto;
}

.empty {
  color: #76829a;
  font-size: 13px;
}

.message {
  border: 1px solid #e5eaf3;
  border-radius: 8px;
  padding: 10px 12px;
  background: #ffffff;
}

.message-meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: #64708a;
  font-size: 12px;
}

.message-meta strong {
  color: #172033;
}

.message p {
  margin: 6px 0 0;
  color: #26324a;
  line-height: 1.5;
  word-break: break-word;
}

.composer {
  display: grid;
  gap: 10px;
}
</style>
