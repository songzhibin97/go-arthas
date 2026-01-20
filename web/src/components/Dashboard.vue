<template>
  <div class="dashboard">
    <MetricsChart :metrics="metricsHistory" :connected="connected" />
    <CPUGauge :cpu="metrics?.cpu" />
    <MemoryPanel :memory="metrics?.memory" />
    <GCStats :gc="metrics?.gc" />
    <SystemInfo :info="systemInfo" />
  </div>
</template>

<script>
import { ref, watch } from 'vue'
import MetricsChart from './MetricsChart.vue'
import CPUGauge from './CPUGauge.vue'
import MemoryPanel from './MemoryPanel.vue'
import GCStats from './GCStats.vue'
import SystemInfo from './SystemInfo.vue'

export default {
  name: 'Dashboard',
  components: {
    MetricsChart,
    CPUGauge,
    MemoryPanel,
    GCStats,
    SystemInfo
  },
  props: {
    metrics: {
      type: Object,
      default: null
    },
    systemInfo: {
      type: Object,
      default: null
    },
    connected: {
      type: Boolean,
      required: true
    }
  },
  setup(props) {
    const metricsHistory = ref([])
    const MAX_HISTORY = 300 // 5 分钟的数据（每秒一个点）

    watch(() => props.metrics, (newMetrics) => {
      if (newMetrics) {
        metricsHistory.value.push({
          timestamp: new Date(newMetrics.timestamp),
          goroutines: newMetrics.goroutines
        })
        
        // 保持最多 5 分钟的历史数据
        if (metricsHistory.value.length > MAX_HISTORY) {
          metricsHistory.value.shift()
        }
      }
    })

    return {
      metricsHistory
    }
  }
}
</script>
