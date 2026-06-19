<template>
  <div class="panel goroutine-panel">
    <div class="panel-header">
      <h2>Goroutines</h2>
      <button class="refresh-btn" @click="refresh" :disabled="loading">
        {{ loading ? '加载中…' : '刷新' }}
      </button>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div v-if="dump">
      <p class="total">
        总数: <strong>{{ dump.total }}</strong>
        <span class="ts">（{{ formatTime(dump.timestamp) }}）</span>
      </p>

      <table class="state-table">
        <thead>
          <tr><th>状态</th><th>数量</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in sortedStates" :key="row.state">
            <td>{{ row.state }}</td>
            <td>{{ row.count }}</td>
          </tr>
        </tbody>
      </table>

      <div v-if="dump.suspected_blocked && dump.suspected_blocked.length" class="suspected">
        <h3>⚠ 疑似阻塞 ({{ dump.suspected_blocked.length }})</h3>
        <div v-for="g in dump.suspected_blocked" :key="g.id" class="suspect">
          <div class="suspect-head">
            goroutine {{ g.id }} [{{ g.state }}, {{ g.wait_minutes }} 分钟]
          </div>
          <pre v-if="g.stack">{{ g.stack }}</pre>
        </div>
      </div>
      <p v-else class="ok">无疑似阻塞 goroutine</p>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue'

export default {
  name: 'GoroutinePanel',
  setup() {
    const dump = ref(null)
    const loading = ref(false)
    const error = ref('')

    const refresh = async () => {
      loading.value = true
      error.value = ''
      try {
        const resp = await fetch('/api/v1/goroutines?min_wait=1')
        if (!resp.ok) throw new Error('HTTP ' + resp.status)
        dump.value = await resp.json()
      } catch (e) {
        error.value = '获取 goroutine 失败: ' + e.message
      } finally {
        loading.value = false
      }
    }

    const sortedStates = computed(() => {
      if (!dump.value || !dump.value.state_counts) return []
      return Object.entries(dump.value.state_counts)
        .map(([state, count]) => ({ state, count }))
        .sort((a, b) => b.count - a.count || a.state.localeCompare(b.state))
    })

    const formatTime = (ts) => {
      try {
        return new Date(ts).toLocaleTimeString()
      } catch {
        return ts
      }
    }

    onMounted(refresh)

    return { dump, loading, error, refresh, sortedStates, formatTime }
  }
}
</script>

<style scoped>
.goroutine-panel {
  background: #fff;
  border-radius: 8px;
  padding: 1rem;
  margin-top: 1rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.refresh-btn {
  padding: 0.3rem 0.8rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: #f7f7f7;
  cursor: pointer;
}
.refresh-btn:disabled {
  opacity: 0.6;
  cursor: default;
}
.total {
  margin: 0.5rem 0;
}
.ts {
  color: #888;
  font-size: 0.85em;
}
.state-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5rem 0;
}
.state-table th,
.state-table td {
  text-align: left;
  padding: 0.3rem 0.6rem;
  border-bottom: 1px solid #eee;
}
.suspected {
  margin-top: 1rem;
}
.suspect {
  margin: 0.5rem 0;
}
.suspect-head {
  font-weight: bold;
  color: #b00;
}
.suspect pre {
  background: #f6f6f6;
  padding: 0.5rem;
  overflow: auto;
  font-size: 0.8em;
}
.error {
  color: #b00;
}
.ok {
  color: #2a2;
}
</style>
