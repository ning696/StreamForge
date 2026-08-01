<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { AttachableTrack, ParticipantTile } from '@/livekit/connection'

const props = defineProps<{
  participant: ParticipantTile
}>()

const videoElement = ref<HTMLVideoElement | null>(null)
let attachedElement: HTMLVideoElement | null = null
let attachedTrack: AttachableTrack | null = null

onMounted(attachTrack)
watch(() => props.participant.screenShareTrack, attachTrack)
onBeforeUnmount(detachTrack)

async function attachTrack() {
  detachTrack()
  await nextTick()
  const element = videoElement.value
  const track = props.participant.screenShareTrack
  if (!element || !track) {
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
  <article class="screen-share-stage">
    <video ref="videoElement" class="screen-share-video" autoplay playsinline />
    <footer class="screen-share-label">
      {{ participant.participantName }} 的屏幕共享
    </footer>
  </article>
</template>
