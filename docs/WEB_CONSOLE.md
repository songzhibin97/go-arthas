# Web Console 使用指南

Go-Arthas Web Console 提供实时可视化仪表板，用于监控和诊断 Go 应用程序。

## ⚠️ 重要说明

**功能范围**：Web Console 专注于**运行时指标监控和性能分析**，提供：
- 实时 Goroutine 数量趋势
- 内存使用情况（堆、栈）
- CPU 使用率监控
- GC 统计信息
- 系统信息展示

**不提供**（受 Go 语言特性限制）：
- 方法级调用追踪（如 Java Arthas 的 `trace`）
- 方法参数/返回值观察（如 Java Arthas 的 `watch`）
- 方法 QPS/RT 监控（如 Java Arthas 的 `monitor`）
- 代码热更新（如 Java Arthas 的 `redefine`）

**替代方案**：
- 调用链追踪：[OpenTelemetry](https://opentelemetry.io/)
- 方法监控：[Prometheus](https://prometheus.io/) + 手动埋点
- 分布式追踪：[Jaeger](https://www.jaegertracing.io/)

## 功能特性

- **实时图表**: Goroutine 数量、内存使用、CPU 使用率的实时趋势图
- **自动更新**: 每秒自动更新，无需手动刷新
- **WebSocket 连接**: 低延迟的实时数据推送
- **响应式设计**: 支持桌面和移动设备
- **自动重连**: 连接断开时自动重连

## 安装和启动

### 开发模式

```bash
cd web
npm install
npm run dev
```

Web Console 将在 `http://localhost:3000` 启动。

### 生产构建

```bash
cd web
npm install
npm run build
```

构建产物在 `dist/` 目录，可以部署到任何静态文件服务器。

### 使用 Docker

```bash
docker build -t go-arthas-web ./web
docker run -p 3000:80 go-arthas-web
```

## 连接到 Agent

### 方法 1: URL 参数

在浏览器中打开：
```
http://localhost:3000/?agent=<host:port>
```

示例：
```
http://localhost:3000/?agent=localhost:8563
http://localhost:3000/?agent=192.168.1.100:8563
```

### 方法 2: 界面输入

1. 打开 `http://localhost:3000`
2. 在连接设置中输入 Agent 地址
3. 点击"连接"按钮

## 界面组件

### 1. 连接状态指示器

位于页面顶部，显示与 Agent 的连接状态：

- **已连接**: WebSocket 连接正常
- **连接中**: 正在建立连接
- **已断开**: 连接断开，正在尝试重连

### 2. Goroutine 趋势图

显示过去 5 分钟的 goroutine 数量变化。

**用途**:
- 监控 goroutine 数量趋势
- 识别 goroutine 泄漏（持续增长）
- 观察负载变化

**交互**:
- 鼠标悬停查看具体数值
- 缩放和平移图表

### 3. 内存面板

显示当前内存使用情况的详细分解。

**指标**:
- **Heap Allocated**: 堆已分配内存
- **Heap In Use**: 堆正在使用的内存
- **Heap Idle**: 堆空闲内存
- **Heap Released**: 已释放给操作系统的内存
- **Stack In Use**: 栈正在使用的内存
- **Total Allocated**: 累计分配的内存
- **System Memory**: 从操作系统获取的总内存

**可视化**:
- 饼图显示内存分布
- 进度条显示使用率
- 数值显示具体大小

### 4. CPU 仪表盘

显示当前 CPU 使用率。

**特性**:
- 仪表盘样式显示
- 颜色编码（绿色: <50%, 黄色: 50-80%, 红色: >80%）
- 实时更新

### 5. GC 统计面板

显示垃圾回收统计信息。

**指标**:
- **Total GC Count**: 总 GC 次数
- **Total Pause Time**: 总暂停时间
- **Last Pause Time**: 最后一次 GC 暂停时间
- **Average Pause Time**: 平均 GC 暂停时间

**用途**:
- 监控 GC 频率
- 识别 GC 暂停时间过长的问题
- 评估内存管理效率

### 6. 系统信息面板

显示应用程序和系统信息。

**信息**:
- Go 版本
- 操作系统
- 架构
- CPU 核心数
- 进程 ID
- 启动时间
- 运行时长

## 使用场景

### 场景 1: 实时监控

**目标**: 持续监控应用程序健康状况

**步骤**:
1. 连接到 Agent
2. 观察各项指标的实时变化
3. 注意异常趋势（如 goroutine 持续增长、内存持续增长）

**关注点**:
- Goroutine 数量是否稳定
- 内存使用是否在合理范围
- CPU 使用率是否正常
- GC 暂停时间是否可接受

### 场景 2: 诊断 Goroutine 泄漏

**症状**: Goroutine 趋势图持续上升

**步骤**:
1. 观察 Goroutine 趋势图，确认持续增长
2. 记录当前 goroutine 数量
3. 等待一段时间（如 5 分钟）
4. 如果数量持续增长，使用 CLI 捕获 goroutine profile：
   ```bash
   go-arthas profile goroutine --host <agent-host:port>
   ```
5. 分析 profile 找出泄漏的 goroutine

### 场景 3: 诊断内存泄漏

**症状**: 内存面板显示内存持续增长

**步骤**:
1. 观察内存面板，特别是 Heap Allocated 和 Heap In Use
2. 记录当前内存使用量
3. 等待一段时间（如 10 分钟）
4. 如果内存持续增长且 GC 无法回收，使用 CLI 捕获堆 profile：
   ```bash
   go-arthas profile heap --host <agent-host:port>
   ```
5. 分析 profile 找出内存分配热点

### 场景 4: 监控 GC 性能

**目标**: 评估 GC 对应用程序的影响

**步骤**:
1. 观察 GC 统计面板
2. 关注以下指标：
   - GC 频率（Total GC Count 增长速度）
   - 平均暂停时间（Average Pause Time）
   - 最大暂停时间（Last Pause Time 的峰值）
3. 如果 GC 暂停时间过长（>10ms），考虑优化内存分配

### 场景 5: 负载测试监控

**目标**: 在负载测试期间监控应用程序行为

**步骤**:
1. 启动负载测试前连接 Web Console
2. 开始负载测试
3. 实时观察：
   - Goroutine 数量变化（应该随负载增加而增加，负载结束后回落）
   - 内存使用变化（应该在合理范围内波动）
   - CPU 使用率（应该与负载成正比）
   - GC 频率和暂停时间
4. 记录峰值和异常情况
5. 负载测试结束后，确认资源是否正确释放

## 故障排除

### 无法连接到 Agent

**问题**: 连接状态显示"已断开"

**检查清单**:
1. Agent 是否正在运行？
   ```bash
   curl http://<agent-host:port>/api/v1/info
   ```
2. Agent 地址是否正确？
3. 防火墙是否阻止了连接？
4. 浏览器控制台是否有错误消息？

**解决方案**:
- 确认 Agent 地址格式正确（`host:port`）
- 检查网络连接
- 查看浏览器控制台的详细错误信息

### 数据不更新

**问题**: 界面显示但数据不更新

**可能原因**:
1. WebSocket 连接断开
2. Agent 的 `EnableMetrics` 未启用
3. 浏览器标签页被挂起（某些浏览器会挂起后台标签页）

**解决方案**:
- 检查连接状态指示器
- 刷新页面重新连接
- 确认 Agent 配置中 `EnableMetrics: true`

### 图表显示异常

**问题**: 图表显示不正常或数据跳跃

**可能原因**:
1. 网络延迟导致数据包丢失
2. Agent 重启导致数据重置
3. 浏览器性能问题

**解决方案**:
- 刷新页面
- 检查网络连接质量
- 使用性能更好的浏览器

### 连接频繁断开

**问题**: 连接状态在"已连接"和"已断开"之间频繁切换

**可能原因**:
1. 网络不稳定
2. 代理或负载均衡器超时设置过短
3. Agent 资源不足

**解决方案**:
- 检查网络稳定性
- 调整代理/负载均衡器的 WebSocket 超时设置
- 检查 Agent 所在服务器的资源使用情况

## 配置选项

### 环境变量

在构建 Web Console 时可以配置以下环境变量：

```bash
# 默认 Agent 地址
VITE_DEFAULT_AGENT_HOST=localhost:8563

# WebSocket 重连间隔（毫秒）
VITE_RECONNECT_INTERVAL=5000

# 图表历史数据点数
VITE_CHART_MAX_POINTS=300
```

### 运行时配置

在 `src/config.js` 中修改配置：

```javascript
export default {
  // 默认 Agent 地址
  defaultAgentHost: 'localhost:8563',
  
  // WebSocket 重连间隔（毫秒）
  reconnectInterval: 5000,
  
  // 图表更新间隔（毫秒）
  chartUpdateInterval: 1000,
  
  // 图表历史数据点数（5 分钟 @ 1 秒间隔 = 300 点）
  chartMaxPoints: 300,
}
```

## 浏览器兼容性

Web Console 支持以下浏览器：

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+
- Opera 76+

**注意**: 需要浏览器支持 WebSocket 和 ES6+。

## 性能优化

### 减少数据传输

如果网络带宽有限，可以：
1. 减少图表历史数据点数
2. 增加更新间隔
3. 禁用不需要的图表

### 提高渲染性能

如果浏览器性能不足，可以：
1. 关闭其他标签页
2. 使用硬件加速
3. 减少图表动画效果

## 部署

### 静态文件服务器

```bash
# 构建
npm run build

# 使用 nginx 部署
cp -r dist/* /var/www/html/go-arthas/

# nginx 配置
server {
    listen 80;
    server_name console.example.com;
    root /var/www/html/go-arthas;
    index index.html;
    
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### Docker 部署

```dockerfile
FROM nginx:alpine
COPY dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### CDN 部署

将构建产物上传到 CDN（如 AWS S3 + CloudFront、阿里云 OSS + CDN）。

## 安全考虑

1. **访问控制**: 在生产环境中，使用防火墙或反向代理限制对 Web Console 的访问
2. **HTTPS**: 在生产环境中使用 HTTPS 保护数据传输
3. **认证**: 考虑添加认证机制（如 Basic Auth、OAuth）
4. **CORS**: Agent 默认允许所有来源，生产环境中应该限制 CORS 策略

## 最佳实践

1. **专用监控**: 为每个环境（开发、测试、生产）部署独立的 Web Console
2. **书签**: 将常用的 Agent 连接保存为浏览器书签
3. **多窗口**: 同时监控多个应用程序时，使用多个浏览器窗口
4. **截图**: 发现问题时及时截图保存证据
5. **结合 CLI**: Web Console 用于实时监控，CLI 用于深度分析

## 参考

- [Vue.js 文档](https://vuejs.org/)
- [Chart.js 文档](https://www.chartjs.org/)
- [WebSocket API](https://developer.mozilla.org/en-US/docs/Web/API/WebSocket)
