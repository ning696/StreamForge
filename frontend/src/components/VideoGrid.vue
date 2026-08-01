<script setup lang="ts">
import { computed } from 'vue'
import { Loading, VideoCamera } from '@element-plus/icons-vue'
import AudioRenderer from '@/components/AudioRenderer.vue'
import ScreenShareStage from '@/components/ScreenShareStage.vue'
import VideoTile from '@/components/VideoTile.vue'
import {
  getActiveScreenShare,
  getRemoteAudioParticipants,
  type LiveKitStatus,
  type ParticipantTile,
} from '@/livekit/connection'

const props = defineProps<{
  participants: ParticipantTile[]
  status: LiveKitStatus
}>()

const hasParticipants = computed(() => props.participants.length > 0)
const activeScreenShare = computed(() => getActiveScreenShare(props.participants))
const remoteAudioParticipants = computed(() => getRemoteAudioParticipants(props.participants))
const gridCountClass = computed(() => `participants-${Math.min(props.participants.length, 8)}`)
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
    return '等待参与者加入'
  }
  return 'LiveKit 媒体尚未连接'
})
</script>

<template>
  <section class="video-grid-shell">
    <div class="audio-renderers" aria-hidden="true">
      <AudioRenderer
        v-for="participant in remoteAudioParticipants"
        :key="participant.participantIdentity"
        :track="participant.microphoneTrack!"
      />
    </div>

    <div v-if="hasParticipants && activeScreenShare" class="screen-share-layout">
      <ScreenShareStage :participant="activeScreenShare" />
      <div class="screen-participant-strip">
        <VideoTile
          v-for="participant in participants"
          :key="participant.participantIdentity"
          :participant="participant"
        />
      </div>
    </div>

    <div v-else-if="hasParticipants" class="video-grid" :class="gridCountClass">
      <VideoTile
        v-for="participant in participants"
        :key="participant.participantIdentity"
        :participant="participant"
      />
    </div>

    <div v-else class="video-empty-state">
      <el-icon>
        <Loading v-if="status === 'connecting' || status === 'reconnecting'" />
        <VideoCamera v-else />
      </el-icon>
      <h2>{{ waitingText }}</h2>
      <p>连接成功后，房间参与者会按固定顺序显示在这里。</p>
    </div>
  </section>
</template>
