<template>
  <div class="card">
    <h2>系统信息</h2>
    <div v-if="info">
      <div class="metric-row">
        <span class="metric-label">Go 版本</span>
        <span class="metric-value">{{ info.go_version }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">操作系统</span>
        <span class="metric-value">{{ info.goos }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">架构</span>
        <span class="metric-value">{{ info.goarch }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">CPU 核心数</span>
        <span class="metric-value">{{ info.num_cpu }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">进程 ID</span>
        <span class="metric-value">{{ info.process_id }}</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">运行时长</span>
        <span class="metric-value">{{ info.uptime }}</span>
      </div>
    </div>
    <div v-else class="metric-row">
      <span class="metric-label">暂无数据</span>
    </div>
  </div>
</template>

<script>
export default {
  name: 'SystemInfo',
  props: {
    info: {
      type: Object,
      default: null
    }
  }
}
</script>
