<template>
  <div class="card" style="margin-top: 20px;">
    <div :class="['status-indicator', connected ? 'status-connected' : 'status-disconnected']">
      <span class="status-dot"></span>
      <span>{{ connected ? '已连接' : '未连接' }}</span>
    </div>
  </div>
</template>

<script>
export default {
  name: 'ConnectionStatus',
  props: {
    connected: {
      type: Boolean,
      required: true
    }
  }
}
</script>
