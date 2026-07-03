# StreamForge WebRTC / SFU 媒体设计

## 1. 设计目标

MVP 阶段直接采用 SFU 架构，不实现 P2P 中间态。媒体服务负责接收每个参与者的上行音视频轨道，并转发给同一房间内的其他参与者。

核心目标：

- 支持单房间 4-8 人音视频通话。
- 支持屏幕共享。
- 支持静音、关闭摄像头、切换设备。
- 支持 Go 媒体服务多实例部署。
- 保证同一房间固定在同一个媒体实例中完成媒体转发。

## 2. MVP 媒体边界

MVP 做：

- 浏览器 `getUserMedia` 获取麦克风和摄像头。
- 浏览器 `getDisplayMedia` 获取屏幕共享。
- WebSocket 交换 SDP 和 ICE Candidate。
- Go 媒体实例使用 pion/webrtc 建立 PeerConnection。
- Go 媒体实例把参与者发布的 Track 转发给同房间其他参与者。

MVP 不做：

- P2P 直连通话。
- MCU 混流。
- 服务端视频转码。
- Simulcast / SVC。
- 自适应订阅策略。
- 同一房间跨多个媒体实例级联。
- 录制与回放。

## 3. 媒体实例与房间关系

```text
media-service-1
  Room A
    Peer A1
    Peer A2
    Peer A3

media-service-2
  Room B
    Peer B1
    Peer B2
```

规则：

- 房间创建时分配媒体实例。
- 房间加入时复用已有媒体实例。
- 房间内媒体转发只发生在所属媒体实例进程内。
- Redis 保存 `roomId -> mediaInstanceId`。
- WebRTC 对象只保存在媒体实例内存中。

## 4. PeerConnection 模型

MVP 采用每个参与者一个 `RTCPeerConnection` 到 SFU 的模型。

每个 PeerConnection 承载：

- 本地上行音频 Track。
- 本地上行摄像头视频 Track。
- 可选屏幕共享视频 Track。
- 来自同房间其他参与者的下行 Track。

浏览器侧：

```text
Local MediaStream
  audio track
  camera video track
  optional screen video track
      │
      ▼
RTCPeerConnection
      │
      ▼
Go SFU media instance
```

服务端侧：

```text
Peer A publishes audio/video
  -> SFU stores Track publication
  -> SFU creates local tracks for Peer B/C
  -> Peer B/C receive remote streams
```

## 5. Track 类型

| Track 类型 | 来源 | 说明 |
|:---|:---|:---|
| `audio` | `getUserMedia` | 麦克风音频 |
| `camera` | `getUserMedia` | 摄像头视频 |
| `screen` | `getDisplayMedia` | 屏幕共享视频 |

Track 元数据：

```json
{
  "trackId": "track-001",
  "peerId": "peer-abc",
  "userId": 1,
  "kind": "video",
  "source": "camera",
  "enabled": true
}
```

## 6. 编解码策略

MVP 优先使用浏览器和 pion/webrtc 默认支持的编解码能力。

推荐目标：

- 音频：Opus。
- 视频：VP8。

如果浏览器协商出 H.264 且服务端支持，可以接受，但 MVP 不为 H.264 做额外优化。

## 7. ICE 和 NAT 穿越

MVP 配置：

- 本地开发支持 `localhost`。
- 默认提供可配置 STUN。
- TURN 作为部署配置项保留，但 MVP 不强制实现 TURN 服务。

示例 ICE 配置：

```json
{
  "iceServers": [
    {
      "urls": ["stun:stun.l.google.com:19302"]
    }
  ]
}
```

生产部署注意：

- HTTPS 是浏览器调用摄像头、麦克风、屏幕共享的必要条件之一，`localhost` 例外。
- WebSocket 可以通过 Ingress 代理。
- WebRTC 媒体 UDP 端口需要单独规划，不能只依赖普通 HTTP Ingress。

## 8. 协商流程

### 8.1 加入房间后开始协商

```text
1. 前端通过 REST 获取 roomId 对应媒体实例连接信息
2. 前端连接 WebSocket
3. 前端发送 room.join
4. 服务端返回 room.joined 和 room.snapshot
5. 前端获取本地媒体
6. 前端创建 RTCPeerConnection
7. 前端添加本地 Track
8. 前端创建 Offer 并通过 WebSocket 发送 webrtc.offer
9. 服务端创建 PeerConnection 并设置 RemoteDescription
10. 服务端创建 Answer 并返回 webrtc.answer
11. 双方交换 ICE Candidate
12. 连接建立后服务端转发 Track
```

### 8.2 新 Peer 加入

新 Peer 加入后：

- 服务端向房间内其他 Peer 广播 `peer.joined`。
- 新 Peer 通过 `room.snapshot` 获取已有成员。
- 服务端根据当前 Track 发布关系为 PeerConnection 添加下行 Track。
- 必要时触发重新协商。

## 9. 媒体转发

当服务端收到某个 Peer 的上行 Track：

1. 识别 `peerId`、`trackId`、`kind`、`source`。
2. 注册到房间内 Track 发布列表。
3. 为房间内其他 Peer 创建对应的下行 LocalTrack。
4. 从上行 Track 读取 RTP 包并写入下行 LocalTrack。
5. 监听 Track 结束，清理发布关系并通知其他 Peer。

MVP 转发策略：

- 摄像头视频转发给同房间其他所有 Peer。
- 音频转发给同房间其他所有 Peer。
- 屏幕共享视频转发给同房间其他所有 Peer。
- 不做订阅优先级。
- 不做带宽自适应。

## 10. 屏幕共享

屏幕共享流程：

```text
用户点击共享屏幕
  -> 前端调用 getDisplayMedia
  -> 添加 screen track 到 PeerConnection
  -> 发送 screen.share.start
  -> 服务端注册 screen track
  -> 广播 peer.screen_share
```

停止流程：

```text
用户点击停止共享或浏览器停止共享
  -> 前端停止 screen track
  -> 发送 screen.share.stop
  -> 服务端清理 screen track
  -> 广播 peer.screen_share
```

前端展示规则：

- 有屏幕共享时，优先展示屏幕共享画面。
- 多人同时屏幕共享时，MVP 可以展示最近开始共享的屏幕，并在参与者列表显示其他共享状态。

## 11. 媒体控制

静音和关闭摄像头优先在前端本地控制：

```ts
track.enabled = false
```

同时通过 WebSocket 发送 `media.state.update`。服务端更新 Peer 状态并广播 `peer.media_state`。

切换设备流程：

1. 前端枚举设备。
2. 用户选择新设备。
3. 前端获取新 Track。
4. 使用 `RTCRtpSender.replaceTrack` 替换旧 Track。
5. 必要时更新媒体状态。

## 12. 生命周期清理

Peer 离开时必须清理：

- WebSocket 连接。
- PeerConnection。
- 上行 Track 读取循环。
- 下行 Track 写入循环。
- 房间 Peer 列表。
- Track 发布列表。
- 媒体状态。

房间为空时：

- 删除房间内存对象。
- 删除或设置 Redis 房间路由过期。

## 13. 错误处理

| 场景 | MVP 行为 |
|:---|:---|
| 摄像头权限被拒绝 | 前端提示权限错误，允许只加入聊天 |
| 麦克风权限被拒绝 | 前端提示权限错误，允许只开视频或只聊天 |
| 屏幕共享被取消 | 不发送 start，或发送 stop，UI 恢复 |
| ICE 连接失败 | 前端提示连接失败，允许用户重新加入 |
| 媒体实例崩溃 | 当前房间中断，MVP 不做迁移 |
| Redis 不可用 | 新建和加入房间不可用 |

## 14. 验收点

- 两个浏览器标签页可以互相看到视频和听到音频。
- 4 个浏览器标签页加入同一房间时，每个用户能看到其他用户。
- 用户静音后，其他用户 UI 能看到静音状态。
- 用户关闭摄像头后，其他用户 UI 能看到摄像头关闭状态。
- 用户屏幕共享后，其他用户能看到屏幕共享画面。
- 媒体服务运行多个副本时，同一房间所有用户连接到同一个媒体实例。
- Redis 中可以看到 `roomId -> mediaInstanceId` 路由。
