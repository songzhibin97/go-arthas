<template>
  <div id="app">
    <header class="header">
      <div class="container">
        <h1>Go-Arthas Console</h1>
        <p>实时监控和性能分析工具</p>
        <p class="note">专注于运行时指标监控，不提供方法级诊断功能</p>
      </div>
    </header>
    
    <div class="container">
      <ConnectionStatus :connected="connected" />
      <Dashboard 
        :metrics="metrics" 
        :systemInfo="systemInfo"
        :connected="connected"
      />
    </div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted } from 'vue'
import Dashboard from './components/Dashboard.vue'
import ConnectionStatus from './components/ConnectionStatus.vue'
import { connectWebSocket } from './websocket'

export default {
  name: 'App',
  components: {
    Dashboard,
    ConnectionStatus
  },
  setup() {
    const connected = ref(false)
    const metrics = ref(null)
    const systemInfo = ref(null)
    let ws = null

    const handleMetrics = (data) => {
      metrics.value = data
    }

    const handleConnect = () => {
      connected.value = true
    }

    const handleDisconnect = () => {
      connected.value = false
    }

    onMounted(async () => {
      // 获取系统信息
      try {
        const response = await fetch('/api/v1/info')
        if (response.ok) {
          systemInfo.value = await response.json()
        }
      } catch (error) {
        console.error('Failed to fetch system info:', error)
      }

      // 连接 WebSocket
      ws = connectWebSocket({
        onMetrics: handleMetrics,
        onConnect: handleConnect,
        onDisconnect: handleDisconnect
      })
    })

    onUnmounted(() => {
      if (ws) {
        ws.close()
      }
    })

    return {
      connected,
      metrics,
      systemInfo
    }
  }
}
</script>

<style>
.header .note {
  font-size: 0.9em;
  color: #ffd700;
  margin-top: 0.5em;
  font-weight: normal;
}
</style>
