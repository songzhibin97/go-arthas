<template>
  <div class="card">
    <h2>CPU 使用率</h2>
    <div class="gauge-container">
      <div class="gauge-value">{{ cpuPercent }}%</div>
      <div class="gauge-label">当前 CPU 使用率</div>
      <div class="gauge-bar" :style="{ width: '100%', marginTop: '20px' }">
        <div 
          class="gauge-fill" 
          :style="{ 
            width: cpuPercent + '%', 
            height: '20px',
            backgroundColor: cpuColor,
            borderRadius: '10px',
            transition: 'width 0.3s ease'
          }"
        ></div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed } from 'vue'

export default {
  name: 'CPUGauge',
  props: {
    cpu: {
      type: Object,
      default: null
    }
  },
  setup(props) {
    const cpuPercent = computed(() => {
      if (!props.cpu || props.cpu.usage_percent === undefined) {
        return 0
      }
      return Math.round(props.cpu.usage_percent * 10) / 10
    })

    const cpuColor = computed(() => {
      const percent = cpuPercent.value
      if (percent < 50) return '#28a745'
      if (percent < 80) return '#ffc107'
      return '#dc3545'
    })

    return {
      cpuPercent,
      cpuColor
    }
  }
}
</script>
