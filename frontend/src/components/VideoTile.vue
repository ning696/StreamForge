<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Microphone, Mute, VideoCamera } from '@element-plus/icons-vue'
import type { AttachableTrack, ParticipantTile } from '@/livekit/connection'

const props = defineProps<{
  participant: ParticipantTile
}>()

const videoElement = ref<HTMLVideoElement | null>(null)
let attachedElement: HTMLVideoElement | null = null
let attachedTrack: AttachableTrack | null = null

const hasCameraVideo = computed(
  () => Boolean(props.participant.cameraTrack && props.participant.cameraEnabled),
)
const initials = computed(() => {
  const name = props.participant.participantName.trim()
  return name ? [...name].slice(0, 2).join('').toUpperCase() : '?'
})

onMounted(attachTrack)

watch(
  () => [props.participant.cameraTrack, props.participant.cameraEnabled] as const,
  attachTrack,
)

onBeforeUnmount(detachTrack)

async function attachTrack() {
  detachTrack()
  if (!hasCameraVideo.value || !props.participant.cameraTrack) {
    return
  }
  await nextTick()
  const element = videoElement.value
  const track = props.participant.cameraTrack
  if (!element || !track || !props.participant.cameraEnabled) {
    return
  }
  track.attach(element)
  attachedElement = element
  attachedTrack = track
}

function detachTrack() {
  if (attachedElement && attachedTrack) {
    attachedTrack.detach(attachedElement)
  }
  attachedElement = null
  attachedTrack = null
}
</script>

<template>
  <article class="video-tile" :class="{ 'is-local': participant.isLocal }">
    <video
      v-if="hasCameraVideo"
      ref="videoElement"
      class="media-video"
      autoplay
      playsinline
      :muted="participant.isLocal"
    />
    <div v-else class="camera-placeholder">
      <div class="participant-avatar">{{ initials }}</div>
      <span>摄像头已关闭</span>
    </div>

    <footer class="tile-footer">
      <div class="tile-identity">
        <el-icon><VideoCamera /></el-icon>
        <span>{{ participant.participantName }}</span>
      </div>
      <div class="tile-meta">
        <span v-if="participant.isLocal">我</span>
        <span
          class="media-status"
          :class="{ 'is-muted': participant.microphoneMuted || !participant.microphoneEnabled }"
          :title="participant.microphoneEnabled ? '麦克风已开启' : '麦克风已静音'"
        >
          <el-icon>
            <Microphone v-if="participant.microphoneEnabled" />
            <Mute v-else />
          </el-icon>
        </span>
        <span v-if="participant.screenSharing">屏幕共享中</span>
      </div>
    </footer>
  </article>
</template>
