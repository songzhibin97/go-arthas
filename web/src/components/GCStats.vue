<template>
  <div class="card">
    <h2>GC 统计信息</h2>
    <div v-if="gc">
      <div class="metric-row">
        <span class="metric-label">GC 执行次数</span>
        <span class="metric-value">{{ gc.num_gc }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">总暂停时间</span>
        <span class="metric-value">{{ formatDuration(gc.pause_total) }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">最后暂停时间</span>
        <span class="metric-value">{{ formatDuration(gc.last_pause) }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">平均暂停时间</span>
        <span class="metric-value">{{ formatDuration(gc.pause_avg) }}</span>
      </div>
    </div>
    <div v-else class="metric-row">
      <span class="metric-label">暂无数据</span>
    </div>
  </div>
</template>

<script>
export default {
  name: 'GCStats',
  props: {
    gc: {
      type: Object,
      default: null
    }
  },
  setup() {
    const formatDuration = (ns) => {
      if (!ns || ns === 0) return '0 ns'
      
      // ns 是纳秒
      if (ns < 1000) return ns + ' ns'
      if (ns < 1000000) return Math.round(ns / 1000 * 100) / 100 + ' μs'
      if (ns < 1000000000) return Math.round(ns / 1000000 * 100) / 100 + ' ms'
      return Math.round(ns / 1000000000 * 100) / 100 + ' s'
    }

    return {
      formatDuration
    }
  }
}
</script>
