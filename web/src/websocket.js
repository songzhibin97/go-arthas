/**
 * WebSocket 连接管理
 * 实现自动重连逻辑（每 5 秒重试）
 */

const RECONNECT_INTERVAL = 5000 // 5 秒

export function connectWebSocket(options) {
  const { onMetrics, onConnect, onDisconnect } = options
  
  let ws = null
  let reconnectTimer = null
  let shouldReconnect = true
  
  const connect = () => {
    // 确定 WebSocket URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.hostname
    const port = import.meta.env.DEV ? '8563' : window.location.port
    const wsUrl = `${protocol}//${host}:${port}/ws/metrics`
    
    console.log('Connecting to WebSocket:', wsUrl)
    
    try {
      ws = new WebSocket(wsUrl)
      
      ws.onopen = () => {
        console.log('WebSocket connected')
        if (onConnect) onConnect()
        
        // 清除重连定时器
        if (reconnectTimer) {
          clearTimeout(reconnectTimer)
          reconnectTimer = null
        }
      }
      
      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          if (onMetrics) onMetrics(data)
        } catch (error) {
          console.error('Failed to parse metrics:', error)
        }
      }
      
      ws.onerror = (error) => {
        console.error('WebSocket error:', error)
      }
      
      ws.onclose = () => {
        console.log('WebSocket disconnected')
        if (onDisconnect) onDisconnect()
        
        // 自动重连
        if (shouldReconnect && !reconnectTimer) {
          console.log(`Reconnecting in ${RECONNECT_INTERVAL / 1000} seconds...`)
          reconnectTimer = setTimeout(() => {
            reconnectTimer = null
            connect()
          }, RECONNECT_INTERVAL)
        }
      }
    } catch (error) {
      console.error('Failed to create WebSocket:', error)
      if (onDisconnect) onDisconnect()
      
      // 重试连接
      if (shouldReconnect && !reconnectTimer) {
        reconnectTimer = setTimeout(() => {
          reconnectTimer = null
          connect()
        }, RECONNECT_INTERVAL)
      }
    }
  }
  
  // 初始连接
  connect()
  
  // 返回控制对象
  return {
    close: () => {
      shouldReconnect = false
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      if (ws) {
        ws.close()
        ws = null
      }
    }
  }
}
