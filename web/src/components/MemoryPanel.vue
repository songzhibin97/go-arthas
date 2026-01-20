<template>
  <div class="card">
    <h2>内存使用情况</h2>
    <div v-if="memory">
      <div class="metric-row">
        <span class="metric-label">堆已分配</span>
        <span class="metric-value">{{ formatBytes(memory.heap_alloc) }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">堆正在使用</span>
        <span class="metric-value">{{ formatBytes(memory.heap_inuse) }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">堆空闲</span>
        <span class="metric-value">{{ formatBytes(memory.heap_idle) }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">栈正在使用</span>
        <span class="metric-value">{{ formatBytes(memory.stack_inuse) }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">系统总内存</span>
        <span class="metric-value">{{ formatBytes(memory.sys) }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">累计分配</span>
        <span class="metric-value">{{ formatBytes(memory.total_alloc) }}</span>
      </div>
    </div>
    <div v-else class="metric-row">
      <span class="metric-label">暂无数据</span>
    </div>
  </div>
</template>

<script>
export default {
  name: 'MemoryPanel',
  props: {
    memory: {
      type: Object,
      default: null
    }
  },
  setup() {
    const formatBytes = (bytes) => {
      if (!bytes || bytes === 0) return '0 B'
      const k = 1024
      const sizes = ['B', 'KB', 'MB', 'GB']
      const i = Math.floor(Math.log(bytes) / Math.log(k))
      return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
    }

    return {
      formatBytes
    }
  }
}
</script>
