# Go-Arthas Web Console

基于 Vue 3 + Vite 的实时监控控制台。

## 安装依赖

```bash
npm install
```

## 开发模式

```bash
npm run dev
```

开发服务器将在 http://localhost:3000 启动，并自动代理 API 请求到 http://localhost:8563

## 构建生产版本

```bash
npm run build
```

构建输出将在 `dist` 目录中。

## 功能特性

- **实时指标监控**: 通过 WebSocket 实时接收和显示运行时指标
- **Goroutine 趋势图**: 显示最近 5 分钟的 goroutine 数量变化
- **内存使用情况**: 详细的内存分配和使用统计
- **CPU 使用率**: 实时 CPU 使用率显示
- **GC 统计**: 垃圾回收次数和暂停时间统计
- **系统信息**: Go 版本、操作系统、架构等信息
- **自动重连**: WebSocket 断开后每 5 秒自动重连

## 架构

- **Vue 3**: 使用 Composition API
- **Chart.js**: 用于绘制实时图表
- **WebSocket**: 实时数据推送
- **Vite**: 快速的开发服务器和构建工具
