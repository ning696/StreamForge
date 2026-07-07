# StreamForge WebSocket 信令协议

## 1. 协议目标

`livekit-standalone-cluster` 分支中，StreamForge 的 WebSocket 不再承载 WebRTC 底层信令。SDP Offer/Answer、ICE Candidate、Track 发布订阅和媒体状态由 LiveKit SDK 与 LiveKit Server 处理。

本文档覆盖 StreamForge 自有实时业务消息：

- 房间业务连接。
- 房间内文字聊天。
- 可选在线状态摘要。
- 错误响应。

LiveKit 连接信息通过 REST 创建/加入房间接口返回，前端随后使用 `livekit-client` 直接连接 LiveKit。

## 2. REST 前置入口

### 2.1 创建房间

```http
POST /api/rooms
Content-Type: application/json
```

请求：

```json
{
  "userId": 1,
  "username": "演示用户"
}
```

响应：

```json
{
  "roomId": "839204",
  "livekitRoomName": "streamforge-839204",
  "livekitUrl": "ws://localhost:7880",
  "livekitToken": "eyJhbGciOi...",
  "livekitIdentity": "user-1-a1b2c3d4",
  "appWsUrl": "ws://localhost:8080/ws/rooms/839204"
}
```

行为：

- Go 房间服务生成 `roomId`。
- Go 房间服务生成或记录 `livekitRoomName`。
- Go 房间服务保存可过期业务房间摘要。
- Go 房间服务为创建者签发 LiveKit Token。
- 返回 LiveKit 连接信息和可选 StreamForge WebSocket 地址。

### 2.2 加入房间

```http
POST /api/rooms/{roomId}/join
Content-Type: application/json
```

请求：

```json
{
  "userId": 2,
  "username": "参与者"
}
```

响应：

```json
{
  "roomId": "839204",
  "livekitRoomName": "streamforge-839204",
  "livekitUrl": "ws://localhost:7880",
  "livekitToken": "eyJhbGciOi...",
  "livekitIdentity": "user-1-a1b2c3d4",
  "appWsUrl": "ws://localhost:8080/ws/rooms/839204"
}
```

行为：

- 服务端根据 `roomId` 查询业务房间摘要。
- 房间不存在时返回 `ROOM_NOT_FOUND`。
- LiveKit 配置不可用或 Token 签发失败时返回 `LIVEKIT_UNAVAILABLE`。
- 加入成功后，前端使用 `livekitUrl + livekitToken` 连接 LiveKit。
- 如果使用 Go WebSocket 聊天，前端再连接 `appWsUrl` 并发送 `room.join`。

## 3. LiveKit 连接

前端不通过 StreamForge WebSocket 交换 WebRTC 信令。连接流程：

```text
REST create/join
  -> 获得 livekitUrl、livekitToken 和 livekitIdentity
  -> new Room()
  -> room.connect(livekitUrl, livekitToken)
  -> enableCameraAndMicrophone()
  -> 监听 LiveKit TrackSubscribed / ParticipantConnected 等事件
```

LiveKit Token 由后端签发，前端不得保存或暴露 `LIVEKIT_API_SECRET`。

## 4. StreamForge WebSocket 连接

如果启用 Go WebSocket 聊天，连接地址：

```text
GET /ws/rooms/{roomId}?userId={userId}&username={username}
```

MVP 暂不使用 StreamForge Token。服务端从查询参数和后续 `room.join` 消息中读取用户基本信息。

连接建立后，客户端应先发送 `room.join`，服务端在收到 `room.join` 前只处理 `ping` 和错误响应。

## 5. 统一消息结构

所有 StreamForge WebSocket 消息使用 JSON。

```json
{
  "type": "chat.send",
  "requestId": "req-001",
  "roomId": "839204",
  "peerId": "app-peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {}
}
```

字段说明：

| 字段 | 必填 | 说明 |
|:---|:---:|:---|
| `type` | 是 | 消息类型 |
| `requestId` | 客户端请求是 | 客户端生成的请求 ID，用于匹配响应 |
| `roomId` | 是 | StreamForge 业务房间 ID |
| `peerId` | 加入后是 | StreamForge WebSocket 连接 ID，不等于 LiveKit participant identity |
| `userId` | 是 | 用户 ID |
| `username` | 是 | 用户名 |
| `timestamp` | 服务端消息是 | 毫秒时间戳 |
| `payload` | 是 | 消息载荷 |

## 6. 消息类型总览

| 类型 | 方向 | 说明 |
|:---|:---|:---|
| `room.join` | Client -> Server | 客户端加入 StreamForge 业务实时通道 |
| `room.joined` | Server -> Client | 加入业务通道成功响应 |
| `room.snapshot` | Server -> Client | StreamForge 业务成员摘要；媒体成员以 LiveKit 为准 |
| `room.leave` | Client -> Server | 客户端主动离开业务通道 |
| `peer.joined` | Server -> Client | 有参与者加入业务通道 |
| `peer.left` | Server -> Client | 有参与者离开业务通道 |
| `chat.send` | Client -> Server | 发送聊天消息 |
| `chat.message` | Server -> Client | 广播聊天消息 |
| `ping` | Client -> Server | 客户端心跳 |
| `pong` | Server -> Client | 服务端心跳响应 |
| `error` | Server -> Client | 错误响应 |

以下旧消息类型在 LiveKit 独立集群方案中废弃：

| 旧类型 | 新处理方式 |
|:---|:---|
| `webrtc.offer` | 由 LiveKit SDK 和 LiveKit Server 内部处理 |
| `webrtc.answer` | 由 LiveKit SDK 和 LiveKit Server 内部处理 |
| `webrtc.ice_candidate` | 由 LiveKit SDK 和 LiveKit Server 内部处理 |
| `media.state.update` | 优先由 LiveKit participant/track 事件处理 |
| `screen.share.start` / `screen.share.stop` | 优先由 LiveKit screen share Track 事件处理 |

## 7. 房间业务消息

### 7.1 `room.join`

```json
{
  "type": "room.join",
  "requestId": "req-join-001",
  "roomId": "839204",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "clientType": "web",
    "displayName": "演示用户",
    "livekitIdentity": "user-1"
  }
}
```

服务端行为：

- 校验业务房间是否存在。
- 生成 StreamForge WebSocket `peerId`。
- 建立临时业务 Peer 对象。
- 返回 `room.joined`。
- 广播 `peer.joined`。
- 发送 `room.snapshot`。

### 7.2 `room.joined`

```json
{
  "type": "room.joined",
  "requestId": "req-join-001",
  "roomId": "839204",
  "peerId": "app-peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {
    "livekitRoomName": "streamforge-839204",
    "livekitIdentity": "user-1-a1b2c3d4"
  }
}
```

### 7.3 `room.snapshot`

```json
{
  "type": "room.snapshot",
  "roomId": "839204",
  "timestamp": 1783072800000,
  "payload": {
    "peers": [
      {
        "peerId": "app-peer-abc",
        "userId": 1,
        "username": "演示用户",
        "livekitIdentity": "user-1"
      }
    ]
  }
}
```

### 7.4 `room.leave`

```json
{
  "type": "room.leave",
  "requestId": "req-leave-001",
  "roomId": "839204",
  "peerId": "app-peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {}
}
```

服务端关闭该 WebSocket 业务连接，并广播 `peer.left`。LiveKit 媒体连接由前端 `room.disconnect()` 关闭。

## 8. 聊天消息

### 8.1 `chat.send`

```json
{
  "type": "chat.send",
  "requestId": "req-chat-001",
  "roomId": "839204",
  "peerId": "app-peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "content": "大家好"
  }
}
```

校验规则：

- `content` 去除首尾空白后不能为空。
- `content` 最大长度 1000 字符。

### 8.2 `chat.message`

```json
{
  "type": "chat.message",
  "roomId": "839204",
  "peerId": "app-peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {
    "messageId": "msg-001",
    "content": "大家好"
  }
}
```

MVP 不持久化聊天消息。服务端只在房间内广播。

## 9. 心跳

客户端每 20 秒发送一次：

```json
{
  "type": "ping",
  "requestId": "req-ping-001",
  "roomId": "839204",
  "peerId": "app-peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {}
}
```

服务端响应：

```json
{
  "type": "pong",
  "requestId": "req-ping-001",
  "roomId": "839204",
  "peerId": "app-peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {}
}
```

## 10. 错误响应

```json
{
  "type": "error",
  "requestId": "req-join-001",
  "roomId": "839204",
  "timestamp": 1783072800000,
  "payload": {
    "code": "ROOM_NOT_FOUND",
    "message": "房间不存在"
  }
}
```

错误码：

| 错误码 | 说明 |
|:---|:---|
| `BAD_REQUEST` | 请求格式错误或参数缺失 |
| `ROOM_NOT_FOUND` | StreamForge 业务房间不存在 |
| `LIVEKIT_UNAVAILABLE` | LiveKit 配置不可用或 Token 签发失败 |
| `PEER_NOT_JOINED` | Peer 尚未完成 `room.join` |
| `MESSAGE_TOO_LONG` | 聊天消息过长或为空 |
| `INTERNAL_ERROR` | 服务端内部错误 |

## 11. 兼容性规则

- 创建/加入房间响应从旧的 `mediaInstanceId/wsUrl/rtcConfig` 改为 `livekitRoomName/livekitUrl/livekitToken/livekitIdentity/appWsUrl`，这是阶段二的有意破坏性变更。
- WebRTC 底层消息类型由 LiveKit 接管，StreamForge 前端不再发送 `webrtc.offer`、`webrtc.answer`、`webrtc.ice_candidate`。
- WebSocket 消息类型必须和前端 TypeScript 类型保持一致。
