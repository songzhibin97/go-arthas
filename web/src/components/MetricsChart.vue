<template>
  <div class="card" style="grid-column: 1 / -1;">
    <h2>Goroutine 数量趋势</h2>
    <div class="chart-container">
      <canvas ref="chartCanvas"></canvas>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, watch, onUnmounted } from 'vue'
import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  TimeScale,
  Title,
  Tooltip,
  Legend
} from 'chart.js'
import 'chartjs-adapter-date-fns'

// 注册 Chart.js 组件
Chart.register(
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  TimeScale,
  Title,
  Tooltip,
  Legend
)

export default {
  name: 'MetricsChart',
  props: {
    metrics: {
      type: Array,
      default: () => []
    },
    connected: {
      type: Boolean,
      required: true
    }
  },
  setup(props) {
    const chartCanvas = ref(null)
    let chart = null

    onMounted(() => {
      if (chartCanvas.value) {
        const ctx = chartCanvas.value.getContext('2d')
        chart = new Chart(ctx, {
          type: 'line',
          data: {
            datasets: [{
              label: 'Goroutines',
              data: [],
              borderColor: '#667eea',
              backgroundColor: 'rgba(102, 126, 234, 0.1)',
              borderWidth: 2,
              fill: true,
              tension: 0.4,
              pointRadius: 0,
              pointHoverRadius: 4
            }]
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
              x: {
                type: 'time',
                time: {
                  unit: 'minute',
                  displayFormats: {
                    minute: 'HH:mm'
                  }
                },
                title: {
                  display: true,
                  text: '时间'
                }
              },
              y: {
                beginAtZero: true,
                title: {
                  display: true,
                  text: 'Goroutine 数量'
                }
              }
            },
            plugins: {
              legend: {
                display: false
              },
              tooltip: {
                mode: 'index',
                intersect: false
              }
            },
            interaction: {
              mode: 'nearest',
              axis: 'x',
              intersect: false
            }
          }
        })
      }
    })

    watch(() => props.metrics, (newMetrics) => {
      if (chart && newMetrics) {
        chart.data.datasets[0].data = newMetrics.map(m => ({
          x: m.timestamp,
          y: m.goroutines
        }))
        chart.update('none') // 使用 'none' 模式避免动画，提高性能
      }
    }, { deep: true })

    onUnmounted(() => {
      if (chart) {
        chart.destroy()
      }
    })

    return {
      chartCanvas
    }
  }
}
</script>
