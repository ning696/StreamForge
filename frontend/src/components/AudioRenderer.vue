<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { AttachableTrack } from '@/livekit/connection'

const props = defineProps<{
  track: AttachableTrack
}>()

const audioElement = ref<HTMLAudioElement | null>(null)
let attachedElement: HTMLAudioElement | null = null
let attachedTrack: AttachableTrack | null = null

onMounted(attachTrack)
watch(() => props.track, attachTrack)
onBeforeUnmount(detachTrack)

async function attachTrack() {
  detachTrack()
  await nextTick()
  const element = audioElement.value
  if (!element) {
    return
  }
  props.track.attach(element)
  attachedElement = element
  attachedTrack = props.track
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
  <audio ref="audioElement" class="hidden-audio" autoplay playsinline />
</template>
