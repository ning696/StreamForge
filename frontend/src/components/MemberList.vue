<script setup lang="ts">
import { UserFilled } from '@element-plus/icons-vue'
import type { PeerState } from '@/types'

defineProps<{
  peers: PeerState[]
}>()
</script>

<template>
  <section class="member-list">
    <header class="panel-title">成员 {{ peers.length }}</header>
    <div class="members">
      <p v-if="peers.length === 0" class="empty">等待成员加入</p>
      <article v-for="peer in peers" :key="peer.peerId" class="member">
        <el-icon><UserFilled /></el-icon>
        <div>
          <strong>{{ peer.username }}</strong>
          <span>{{ peer.peerId }}</span>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.member-list {
  display: grid;
  gap: 12px;
}

.panel-title {
  font-size: 15px;
  font-weight: 700;
  color: #172033;
}

.members {
  display: grid;
  gap: 8px;
}

.empty {
  color: #76829a;
  font-size: 13px;
}

.member {
  display: grid;
  grid-template-columns: 32px 1fr;
  align-items: center;
  gap: 10px;
  border: 1px solid #e5eaf3;
  border-radius: 8px;
  padding: 9px 10px;
  background: #ffffff;
}

.member .el-icon {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #e8f7f4;
  color: #0f8f7c;
}

.member strong,
.member span {
  display: block;
}

.member strong {
  color: #172033;
  font-size: 14px;
}

.member span {
  color: #76829a;
  font-size: 12px;
}
</style>
