<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Microphone, VideoCamera } from '@element-plus/icons-vue'
import type { MediaTile } from '@/livekit/connection'

const props = defineProps<{
  tile: MediaTile
}>()

const mediaElement = ref<HTMLVideoElement | HTMLAudioElement | null>(null)
let attachedElement: HTMLMediaElement | null = null

const isVideo = computed(() => props.tile.kind === 'video')
const sourceLabel = computed(() => {
  if (props.tile.source === 'screen') {
    return '屏幕共享'
  }
  if (props.tile.source === 'microphone') {
    return '麦克风'
  }
  if (props.tile.source === 'camera') {
    return '摄像头'
  }
  return '媒体'
})

onMounted(() => {
  attachTrack()
})

watch(
  () => props.tile.track,
  () => {
    attachTrack()
  },
)

onBeforeUnmount(() => {
  detachTrack()
})

async function attachTrack() {
  await nextTick()
  const element = mediaElement.value
  if (!element || !props.tile.track) {
    return
  }
  detachTrack()
  props.tile.track.attach(element)
  attachedElement = element
}

function detachTrack() {
  if (attachedElement && props.tile.track) {
    props.tile.track.detach(attachedElement)
  }
  attachedElement = null
}
</script>

<template>
  <article class="video-tile" :class="{ 'is-local': tile.isLocal, 'is-audio': !isVideo }">
    <video
      v-if="isVideo"
      ref="mediaElement"
      class="media-video"
      autoplay
      playsinline
      :muted="tile.isLocal"
    />
    <template v-else>
      <audio ref="mediaElement" autoplay playsinline :muted="tile.isLocal" />
      <div class="audio-placeholder">
        <el-icon><Microphone /></el-icon>
      </div>
    </template>

    <footer class="tile-footer">
      <div class="tile-identity">
        <el-icon><VideoCamera v-if="isVideo" /><Microphone v-else /></el-icon>
        <span>{{ tile.participantName }}</span>
      </div>
      <div class="tile-meta">
        <span v-if="tile.isLocal">我</span>
        <span>{{ sourceLabel }}</span>
        <span v-if="tile.muted">已静音</span>
      </div>
    </footer>
  </article>
</template>