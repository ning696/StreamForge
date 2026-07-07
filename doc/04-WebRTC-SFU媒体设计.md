# StreamForge WebRTC / SFU 媒体设计

## 1. 设计目标

MVP 阶段直接采用 SFU 架构，不实现 P2P 中间态。`livekit-standalone-cluster` 分支使用 LiveKit 独立集群承载 WebRTC 与 SFU 能力，StreamForge Go 服务不自研媒体转发。

核心目标：

- 支持单房间 4-8 人音视频通话。
- 支持屏幕共享。
- 支持静音、关闭摄像头、切换设备。
- 使用 LiveKit 处理 WebRTC 信令、ICE、Track 发布订阅和 SFU 转发。
- StreamForge 只负责业务房间和 LiveKit Token 签发。

## 2. MVP 媒体边界

MVP 做：

- 前端使用 `livekit-client` 连接 LiveKit 房间。
- 前端使用 LiveKit SDK 打开摄像头和麦克风。
- 前端使用 LiveKit SDK 打开屏幕共享。
- LiveKit Server 负责 WebRTC 信令、ICE、SFU 转发和参与者媒体状态。
- Go 房间服务签发指定房间、指定参与者的 LiveKit Token。

MVP 不做：

- P2P 直连通话。
- MCU 混流。
- StreamForge Go 服务自研 SFU。
- 由 StreamForge Go 服务处理 SDP、ICE Candidate、PeerConnection、RTP/RTCP 或 Track。
- 服务端视频转码。
- 录制与回放。

## 3. LiveKit 房间关系

```text
StreamForge roomId = 839204
  -> livekitRoomName = streamforge-839204

LiveKit Room streamforge-839204
  Participant user-1
    microphone track
    camera track
  Participant user-2
    microphone track
    camera track
```

规则：

- StreamForge 生成业务 `roomId`。
- Go 房间服务将业务 `roomId` 映射为 `livekitRoomName`。
- Go 房间服务为每个用户签发加入该 LiveKit 房间的 Token。
- LiveKit 管理房间内实际媒体连接、Track 和参与者状态。
- StreamForge Redis 不保存 `roomId -> mediaInstanceId` 媒体路由。

## 4. 前端连接模型

浏览器侧不直接创建应用层 `RTCPeerConnection`。前端封装 LiveKit SDK：

```text
REST joinRoom
  -> livekitUrl + livekitToken
  -> new LiveKit Room()
  -> room.connect(livekitUrl, livekitToken)
  -> localParticipant.enableCameraAndMicrophone()
  -> RoomEvent.TrackSubscribed 渲染远端 Track
```

每个参与者通过 LiveKit SDK 发布：

- 麦克风音频。
- 摄像头视频。
- 可选屏幕共享视频。

前端通过 LiveKit 事件订阅：

- 远端参与者加入和离开。
- Track 发布和取消发布。
- Track 订阅和取消订阅。
- 摄像头、麦克风、屏幕共享状态变化。
- 连接断开、重连和错误状态。

## 5. Track 类型

| Track 类型 | LiveKit Source | 来源 | 说明 |
|:---|:---|:---|:---|
| 音频 | `Microphone` | 摄像头/麦克风授权 | 麦克风音频 |
| 摄像头视频 | `Camera` | 摄像头授权 | 用户摄像头画面 |
| 屏幕共享视频 | `ScreenShare` | 屏幕选择授权 | 屏幕、窗口或标签页画面 |

StreamForge 不自定义服务端 Track 元数据。需要展示的用户身份优先来自 LiveKit participant identity、name 和 metadata。

## 6. 编解码策略

MVP 使用 LiveKit 和浏览器默认协商能力。推荐目标：

- 音频：Opus。
- 视频：VP8 或浏览器/LiveKit 默认可用编码。

MVP 不为 H.264、Simulcast、SVC 或带宽自适应做额外策略。若 LiveKit 默认启用相关能力，前端只使用稳定的基础配置。

## 7. ICE、TURN 和 NAT 穿越

MVP 配置：

- 本地开发支持 `localhost`。
- LiveKit 负责向浏览器下发 ICE 配置。
- 部署文档保留 TURN/TLS、公网 IP、RTC UDP 端口配置。

生产部署注意：

- HTTPS 是浏览器调用摄像头、麦克风、屏幕共享的必要条件之一，`localhost` 例外。
- LiveKit 信令可以通过 WebSocket/HTTPS 暴露。
- LiveKit RTC UDP 端口需要单独规划，不能只依赖普通 HTTP Ingress。
- 企业网络或受限网络建议配置 TURN/TLS。

## 8. 协商流程

### 8.1 加入房间后开始媒体连接

```text
1. 前端通过 REST 获取 livekitUrl 和 livekitToken
2. 前端创建 LiveKit Room 实例
3. 前端注册 RoomEvent 监听器
4. 前端调用 room.connect(livekitUrl, livekitToken)
5. 前端启用摄像头和麦克风
6. LiveKit SDK 发布本地 Track
7. LiveKit Server 完成 WebRTC 协商和 SFU 转发
8. 前端收到 TrackSubscribed 事件并渲染远端视频/音频
```

### 8.2 新参与者加入

新参与者加入后：

- LiveKit 向其他客户端触发参与者加入事件。
- 新参与者发布本地 Track。
- 其他客户端收到远端 Track 订阅事件并渲染。
- StreamForge WebSocket 可选地广播业务成员摘要，但媒体成员事实以 LiveKit 为准。

## 9. 媒体转发

媒体转发由 LiveKit 负责：

1. 浏览器向 LiveKit 发布本地 Track。
2. LiveKit 根据房间和订阅关系向其他参与者转发媒体。
3. 前端使用 LiveKit SDK 订阅和渲染远端 Track。
4. Track 结束、取消发布或参与者离开时，LiveKit 触发事件，前端清理 UI。

MVP 转发策略：

- 摄像头视频转发给同房间其他参与者。
- 音频转发给同房间其他参与者。
- 屏幕共享视频转发给同房间其他参与者。
- 不自定义订阅优先级。
- 不自定义带宽自适应策略。

## 10. 屏幕共享

屏幕共享流程：

```text
用户点击共享屏幕
  -> 前端调用 LiveKit SDK 开启屏幕共享
  -> 浏览器弹出屏幕/窗口选择
  -> LiveKit 发布 ScreenShare Track
  -> 其他客户端收到 TrackSubscribed 事件
  -> UI 展示屏幕共享画面
```

停止流程：

```text
用户点击停止共享或浏览器停止共享
  -> LiveKit 取消发布 ScreenShare Track
  -> 其他客户端收到取消订阅或 Track 更新事件
  -> UI 恢复普通视频布局
```

前端展示规则：

- 有屏幕共享时，优先展示屏幕共享画面。
- 多人同时屏幕共享时，MVP 可以展示最近开始共享的屏幕，并在参与者列表显示其他共享状态。

## 11. 媒体控制

媒体控制使用 LiveKit SDK：

- 开启/关闭麦克风。
- 开启/关闭摄像头。
- 切换麦克风。
- 切换摄像头。
- 开启/关闭屏幕共享。

状态变化通过 LiveKit participant 和 Track 事件同步。StreamForge 可在 UI 需要时维护前端状态，不把该状态作为服务端事实来源。

## 12. 生命周期清理

用户离开时：

- 前端调用 LiveKit `room.disconnect()`。
- 前端关闭 Go WebSocket 聊天连接（如果启用）。
- 前端停止本地预览和释放 UI 引用。
- LiveKit 清理媒体连接和 Track。
- Go 房间服务清理临时业务在线摘要。

房间为空时：

- LiveKit 房间可以自然释放。
- StreamForge 业务房间摘要可以删除或设置短 TTL。

## 13. 错误处理

| 场景 | MVP 行为 |
|:---|:---|
| 摄像头权限被拒绝 | 前端提示权限错误，允许只加入聊天或只听音频 |
| 麦克风权限被拒绝 | 前端提示权限错误，允许只开视频或只聊天 |
| 屏幕共享被取消 | 不发布屏幕 Track，UI 恢复 |
| LiveKit Token 无效 | 前端提示加入失败，允许重新进入房间 |
| LiveKit 连接失败 | 前端提示连接失败，允许用户重新加入 |
| LiveKit 服务不可用 | 创建/加入接口返回 `LIVEKIT_UNAVAILABLE` 或前端连接失败 |
| Redis 不可用 | LiveKit 多节点协调或 StreamForge 运行时摘要可能不可用 |

## 14. 验收点

- 两个浏览器标签页可以互相看到视频和听到音频。
- 4 个浏览器标签页加入同一房间时，每个用户能看到其他用户。
- 用户静音后，其他用户 UI 能看到静音状态。
- 用户关闭摄像头后，其他用户 UI 能看到摄像头关闭状态。
- 用户屏幕共享后，其他用户能看到屏幕共享画面。
- Go 房间服务不处理 SDP/ICE/Track，只签发 LiveKit Token。
- 前端使用 `livekit-client` 连接 LiveKit 房间。

