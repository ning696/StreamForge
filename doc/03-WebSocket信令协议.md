# StreamForge WebSocket 信令协议

## 1. 协议目标

信令协议负责房间内实时控制消息，不承载媒体流。媒体流通过 WebRTC 传输，信令通过 WebSocket 传输。

协议覆盖：

- 房间加入和离开。
- 参与者上线和下线通知。
- WebRTC Offer / Answer / ICE Candidate 交换。
- 聊天消息。
- 媒体状态变化。
- 屏幕共享状态变化。
- 房间快照和错误响应。

## 2. REST 前置入口

MVP 中，WebSocket 连接前先通过 REST 获取房间路由和连接信息。

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
  "mediaInstanceId": "media-001",
  "wsUrl": "wss://example.com/ws/rooms/839204",
  "rtcConfig": {
    "iceServers": [
      {
        "urls": ["stun:stun.l.google.com:19302"]
      }
    ]
  }
}
```

行为：

- 服务端生成 `roomId`。
- 服务端选择健康媒体实例。
- 服务端写入 Redis 房间路由。
- 房间创建成功后，返回目标媒体实例连接信息。

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
  "mediaInstanceId": "media-001",
  "wsUrl": "wss://example.com/ws/rooms/839204",
  "rtcConfig": {
    "iceServers": [
      {
        "urls": ["stun:stun.l.google.com:19302"]
      }
    ]
  }
}
```

行为：

- 服务端根据 `roomId` 查询 Redis 路由。
- 路由不存在时返回 `ROOM_NOT_FOUND`。
- 媒体实例不健康时返回 `MEDIA_INSTANCE_UNAVAILABLE`。
- 加入成功后，用户仍需要通过 WebSocket 发送 `room.join` 完成实时连接注册。

## 3. WebSocket 连接

连接地址：

```text
GET /ws/rooms/{roomId}?userId={userId}&username={username}
```

MVP 暂不使用 Token。服务端从查询参数和后续 `room.join` 消息中读取用户基本信息。

连接建立后，客户端必须先发送 `room.join`。服务端在收到 `room.join` 前不处理 WebRTC 协商消息。

## 4. 统一消息结构

所有 WebSocket 消息使用 JSON。

```json
{
  "type": "room.join",
  "requestId": "req-001",
  "roomId": "839204",
  "peerId": "peer-abc",
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
| `roomId` | 是 | 房间 ID |
| `peerId` | 加入后是 | 参与者在房间内的连接 ID |
| `userId` | 是 | 用户 ID |
| `username` | 是 | 用户名 |
| `timestamp` | 服务端消息是 | 毫秒时间戳 |
| `payload` | 是 | 消息载荷 |

服务端广播消息可以没有 `requestId`。

## 5. 消息类型总览

| 类型 | 方向 | 说明 |
|:---|:---|:---|
| `room.join` | Client -> Server | 客户端加入房间实时连接 |
| `room.joined` | Server -> Client | 加入成功响应 |
| `room.snapshot` | Server -> Client | 房间当前成员和状态快照 |
| `room.leave` | Client -> Server | 客户端主动离开房间 |
| `peer.joined` | Server -> Client | 有参与者加入 |
| `peer.left` | Server -> Client | 有参与者离开 |
| `chat.send` | Client -> Server | 发送聊天消息 |
| `chat.message` | Server -> Client | 广播聊天消息 |
| `webrtc.offer` | Client -> Server | 客户端发送 SDP Offer |
| `webrtc.answer` | Server -> Client | 服务端返回 SDP Answer |
| `webrtc.ice_candidate` | 双向 | ICE Candidate 交换 |
| `media.state.update` | Client -> Server | 当前用户媒体状态变化 |
| `peer.media_state` | Server -> Client | 广播参与者媒体状态 |
| `screen.share.start` | Client -> Server | 当前用户开始屏幕共享 |
| `screen.share.stop` | Client -> Server | 当前用户停止屏幕共享 |
| `peer.screen_share` | Server -> Client | 广播屏幕共享状态 |
| `ping` | Client -> Server | 客户端心跳 |
| `pong` | Server -> Client | 服务端心跳响应 |
| `error` | Server -> Client | 错误响应 |

## 6. 房间消息

### 6.1 `room.join`

客户端加入房间实时连接。

```json
{
  "type": "room.join",
  "requestId": "req-join-001",
  "roomId": "839204",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "clientType": "web",
    "displayName": "演示用户"
  }
}
```

服务端行为：

- 校验房间是否在当前媒体实例。
- 生成 `peerId`。
- 建立 Peer 内存对象。
- 返回 `room.joined`。
- 广播 `peer.joined`。
- 发送 `room.snapshot`。

### 6.2 `room.joined`

```json
{
  "type": "room.joined",
  "requestId": "req-join-001",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {
    "mediaInstanceId": "media-001"
  }
}
```

### 6.3 `room.snapshot`

```json
{
  "type": "room.snapshot",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {
    "peers": [
      {
        "peerId": "peer-abc",
        "userId": 1,
        "username": "演示用户",
        "audioEnabled": true,
        "videoEnabled": true,
        "screenSharing": false
      }
    ]
  }
}
```

### 6.4 `room.leave`

```json
{
  "type": "room.leave",
  "requestId": "req-leave-001",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {}
}
```

服务端关闭该 Peer 相关的 WebRTC 和房间状态，并广播 `peer.left`。

## 7. 聊天消息

### 7.1 `chat.send`

```json
{
  "type": "chat.send",
  "requestId": "req-chat-001",
  "roomId": "839204",
  "peerId": "peer-abc",
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

### 7.2 `chat.message`

```json
{
  "type": "chat.message",
  "roomId": "839204",
  "peerId": "peer-abc",
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

## 8. WebRTC 信令

### 8.1 `webrtc.offer`

```json
{
  "type": "webrtc.offer",
  "requestId": "req-offer-001",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "sdp": "v=0...",
    "type": "offer"
  }
}
```

### 8.2 `webrtc.answer`

```json
{
  "type": "webrtc.answer",
  "requestId": "req-offer-001",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {
    "sdp": "v=0...",
    "type": "answer"
  }
}
```

### 8.3 `webrtc.ice_candidate`

```json
{
  "type": "webrtc.ice_candidate",
  "requestId": "req-ice-001",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "candidate": "candidate:...",
    "sdpMid": "0",
    "sdpMLineIndex": 0
  }
}
```

ICE Candidate 可以双向发送。服务端需要在 PeerConnection 未准备好时短暂缓存 Candidate。

## 9. 媒体状态消息

### 9.1 `media.state.update`

```json
{
  "type": "media.state.update",
  "requestId": "req-media-001",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "audioEnabled": false,
    "videoEnabled": true
  }
}
```

### 9.2 `peer.media_state`

```json
{
  "type": "peer.media_state",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {
    "audioEnabled": false,
    "videoEnabled": true
  }
}
```

## 10. 屏幕共享消息

### 10.1 `screen.share.start`

```json
{
  "type": "screen.share.start",
  "requestId": "req-screen-001",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "trackId": "screen-track-001"
  }
}
```

### 10.2 `screen.share.stop`

```json
{
  "type": "screen.share.stop",
  "requestId": "req-screen-002",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "payload": {
    "trackId": "screen-track-001"
  }
}
```

### 10.3 `peer.screen_share`

```json
{
  "type": "peer.screen_share",
  "roomId": "839204",
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {
    "screenSharing": true,
    "trackId": "screen-track-001"
  }
}
```

## 11. 心跳

客户端每 20 秒发送一次：

```json
{
  "type": "ping",
  "requestId": "req-ping-001",
  "roomId": "839204",
  "peerId": "peer-abc",
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
  "peerId": "peer-abc",
  "userId": 1,
  "username": "演示用户",
  "timestamp": 1783072800000,
  "payload": {}
}
```

## 12. 错误响应

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
| `ROOM_NOT_FOUND` | 房间不存在 |
| `ROOM_NOT_ON_INSTANCE` | 当前连接的媒体实例不是该房间所属实例 |
| `MEDIA_INSTANCE_UNAVAILABLE` | 房间所属媒体实例不可用 |
| `PEER_NOT_JOINED` | Peer 尚未完成 `room.join` |
| `INVALID_SDP` | SDP 无效 |
| `INVALID_ICE_CANDIDATE` | ICE Candidate 无效 |
| `MESSAGE_TOO_LONG` | 聊天消息过长 |
| `INTERNAL_ERROR` | 服务端内部错误 |

## 13. 兼容性规则

- 新增消息类型必须向后兼容旧客户端。
- 修改已有 `payload` 字段时只能新增可选字段，不得随意改名。
- 删除字段必须先在文档和代码中标记废弃，再在后续版本移除。
- WebSocket 消息类型必须和前端 TypeScript 类型保持一致。
