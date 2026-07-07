<script setup lang="ts">
import { computed } from 'vue'
import { Loading, VideoCamera } from '@element-plus/icons-vue'
import VideoTile from '@/components/VideoTile.vue'
import type { LiveKitStatus, MediaTile } from '@/livekit/connection'

const props = defineProps<{
  tiles: MediaTile[]
  status: LiveKitStatus
}>()

const hasTiles = computed(() => props.tiles.length > 0)
const waitingText = computed(() => {
  if (props.status === 'connecting') {
    return '正在连接 LiveKit...'
  }
  if (props.status === 'reconnecting') {
    return '正在重新连接 LiveKit...'
  }
  if (props.status === 'failed') {
    return 'LiveKit 连接失败'
  }
  if (props.status === 'connected') {
    return '等待摄像头和麦克风'
  }
  return 'LiveKit 媒体尚未连接'
})
</script>

<template>
  <section class="video-grid-shell">
    <div v-if="hasTiles" class="video-grid" :data-count="tiles.length">
      <VideoTile v-for="tile in tiles" :key="tile.id" :tile="tile" />
    </div>
    <div v-else class="video-empty-state">
      <el-icon>
        <Loading v-if="status === 'connecting' || status === 'reconnecting'" />
        <VideoCamera v-else />
      </el-icon>
      <h2>{{ waitingText }}</h2>
      <p>当 LiveKit 发布或订阅到音视频轨道后，画面会显示在这里。</p>
    </div>
  </section>
</template>