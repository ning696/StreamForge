# StreamForge MVP 需求规格

## 1. 目标

MVP 阶段交付一个可运行的视频会议系统，支持简单用户体系、多人音视频通话、屏幕共享、文字聊天、媒体控制、在线状态、LiveKit 独立集群，以及 Docker Compose 和 Kubernetes 部署。

MVP 的成功标准是：用户可以注册、登录、创建房间、邀请其他用户加入房间，并在同一房间内完成 4-8 人 LiveKit SFU 音视频通话、文字聊天、静音、关闭摄像头和屏幕共享。StreamForge Go 服务负责业务房间和 LiveKit Token 签发，不自研 SFU。

## 2. 用户角色

| 角色 | 说明 |
|:---|:---|
| 普通用户 | 可以注册账号、登录、创建房间、加入房间、进行音视频会议 |
| 房间创建者 | 创建房间的普通用户；MVP 不提供主持人权限和房间管理特权 |
| 参与者 | 已加入房间的用户；可以发送消息、控制自己的麦克风、摄像头和屏幕共享 |

## 3. MVP 功能清单

| 编号 | 功能 | 优先级 | 完成定义 |
|:---|:---|:---:|:---|
| FR-001 | 用户注册 | P0 | 用户输入账号、密码、用户名后创建账号；账号唯一；密码不能明文存储 |
| FR-002 | 登录校验 | P0 | 用户输入账号和密码后，服务端校验成功并返回用户基本信息 |
| FR-003 | 用户信息查询 | P0 | 前端可以根据登录结果展示当前用户 ID、账号和用户名 |
| FR-004 | 创建房间 | P0 | 用户可以创建房间，系统生成房间号、LiveKit 房间名和创建者 LiveKit Token |
| FR-005 | 加入房间 | P0 | 用户输入房间号后加入业务房间，后端签发该用户加入对应 LiveKit 房间的 Token |
| FR-006 | 离开房间 | P0 | 用户主动离开后断开 LiveKit 连接和业务 WebSocket，并通知房间内其他参与者 |
| FR-007 | 多人音视频通话 | P0 | 同一房间支持 4-8 人通过 LiveKit 进行 SFU 音视频通话 |
| FR-008 | 屏幕共享 | P0 | 参与者可以通过 LiveKit SDK 共享屏幕，并可停止共享 |
| FR-009 | 媒体控制 | P0 | 参与者可以静音、取消静音、关闭摄像头、开启摄像头、切换设备 |
| FR-010 | 在线状态 | P0 | 房间内实时显示参与者数量和基本参与者列表 |
| FR-011 | 实时聊天 | P0 | 房间内参与者可以发送文字消息，消息广播给同房间所有在线参与者 |
| FR-012 | LiveKit 独立集群 | P0 | LiveKit 作为独立媒体服务运行，StreamForge 使用后端 Token 签发接入 |
| FR-013 | 本地部署 | P0 | 通过 Docker Compose 启动前端、房间服务、用户服务、LiveKit、MySQL、Redis |
| FR-014 | Kubernetes 部署 | P0 | 提供可运行的 K8s Manifest，包含 LiveKit 信令、RTC 端口和 Redis 配置示例 |

## 4. 用户体系需求

### 4.1 注册

用户注册时输入：

- 账号 `account`
- 密码 `password`
- 用户名 `username`

约束：

- `account` 必填、唯一，长度 4-32。
- `password` 必填，长度 6-64。
- `username` 必填，长度 1-32。
- 密码必须在服务端做哈希后存储，数据库不保存明文密码。
- 账号重复时返回明确错误。

### 4.2 登录校验

用户登录时输入：

- 账号 `account`
- 密码 `password`

登录成功返回：

```json
{
  "userId": 1,
  "account": "demo",
  "username": "演示用户"
}
```

MVP 暂不做 JWT、Session、OAuth2、权限校验。前端可以把登录结果保存在本地状态中，并在创建或加入房间时将 `userId` 和 `username` 传给 Go 房间服务。房间服务在 MVP 阶段信任前端传入的用户基本信息，并用这些信息生成 LiveKit participant identity 和 metadata。

### 4.3 用户信息查询

用户服务提供按 `userId` 查询用户基本信息的接口，返回 `userId`、`account`、`username`。接口不返回密码哈希。

## 5. 房间需求

### 5.1 创建房间

用户创建房间时，Go 房间服务生成唯一 `roomId`，并映射到 LiveKit 房间名：

```text
streamforge-{roomId}
```

创建成功后返回：

- `roomId`
- `livekitRoomName`
- `livekitUrl`
- `livekitToken`
- `livekitIdentity`
- 可选 `appWsUrl`，用于 StreamForge 自有聊天或业务消息

示例响应：

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

### 5.2 加入房间

用户加入房间时，Go 房间服务查询业务房间摘要。

- 如果房间存在，签发该用户加入对应 LiveKit 房间的 Token。
- 如果房间不存在，返回 `ROOM_NOT_FOUND`。
- 如果 LiveKit 配置不可用，返回 `LIVEKIT_UNAVAILABLE`。
- MVP 不做主持人审批、房间密码或权限校验。

### 5.3 离开房间

用户离开房间时：

- 前端断开 LiveKit 房间连接。
- 前端关闭 StreamForge WebSocket（如果启用聊天通道）。
- Go 房间服务从临时在线摘要中移除该用户。
- 房间为空后，Go 房间服务可以删除或设置业务房间摘要短 TTL。
- WebRTC PeerConnection、媒体 Track、重连和媒体状态清理由 LiveKit 负责。

## 6. 音视频需求

### 6.1 SFU 通话

- 每个参与者通过 LiveKit SDK 发布自己的音频和视频轨道。
- LiveKit 不做 MCU 混流，负责把媒体轨道转发给同房间其他参与者。
- MVP 支持 4-8 人同房间音视频通话。
- StreamForge Go 服务不直接处理 SDP、ICE、RTP、RTCP 或 Track。

### 6.2 屏幕共享

- 屏幕共享使用 LiveKit SDK，底层由浏览器 `getDisplayMedia` 获取屏幕轨道。
- 屏幕共享作为独立视频 Track 发布。
- 同一时间允许多个参与者发起屏幕共享，但前端 MVP 可以优先展示最近开始共享的屏幕。
- 参与者停止共享后，前端根据 LiveKit Track 事件更新 UI。

### 6.3 媒体控制

媒体控制优先使用 LiveKit SDK：

- 静音 / 取消静音。
- 关闭摄像头 / 开启摄像头。
- 切换麦克风。
- 切换摄像头。

状态变化通过 LiveKit 参与者和 Track 事件同步到其他客户端。StreamForge 可在需要时保存可过期状态摘要，但不作为媒体状态事实来源。

## 7. 聊天需求

- 聊天消息只在当前房间内广播。
- MVP 不持久化聊天记录。
- 每条聊天消息包含消息 ID、房间 ID、发送者用户 ID、发送者用户名、消息内容和发送时间。
- 消息内容不能为空，最大长度 1000 字符。
- MVP 可以继续使用 Go WebSocket 承载聊天；后续可迁移到 LiveKit data packet。

## 8. LiveKit 集群需求

### 8.1 LiveKit 配置

Go 房间服务需要配置：

- `LIVEKIT_URL`
- `LIVEKIT_API_KEY`
- `LIVEKIT_API_SECRET`
- Token 有效期，MVP 建议 2-24 小时

### 8.2 Token 签发

LiveKit Token 必须由后端签发，不能在前端暴露 API secret。

Token 中至少包含：

- `roomJoin` 权限。
- LiveKit 房间名。
- participant identity，建议使用 `user-{userId}` 或 `user-{userId}-{短随机}`。
- participant name，使用 `username`。
- metadata，保存 `userId`、`username`、`roomId` 等非敏感信息。

### 8.3 LiveKit 多节点

MVP 可以从单个 LiveKit 节点开始，但部署文档需要保留多节点配置路径：

- 多个 LiveKit 节点共享 Redis。
- LiveKit 自己负责媒体节点协调和房间调度。
- StreamForge 不保存 `roomId -> mediaInstanceId` 媒体路由。

## 9. 部署需求

MVP 需要提供：

- 前端 Dockerfile。
- Go 房间服务 Dockerfile。
- Java 用户服务 Dockerfile。
- LiveKit 配置示例。
- docker-compose 配置。
- Kubernetes Deployment / Service / Ingress / ConfigMap / Secret 示例。
- MySQL 初始化脚本。
- Redis 配置说明。

## 10. 非功能需求

| 类别 | 要求 |
|:---|:---|
| 浏览器兼容 | 优先支持最新版 Chrome、Edge；Firefox 作为兼容目标 |
| 通话规模 | MVP 支持单房间 4-8 人 |
| 延迟 | 局域网或良好公网环境下应保持可用的实时互动体验 |
| 可维护性 | 服务边界、协议、数据模型必须有文档对应 |
| 可部署性 | 本地 Docker Compose 和 K8s Manifest 均可启动 MVP 环境 |
| 安全性 | 密码不明文存储；LiveKit API secret 只存在后端；MVP 暂不做完整鉴权 |

## 11. MVP 不包含

- 完整鉴权体系。
- 用户权限、主持人权限、踢人、禁言。
- 房间历史、聊天记录、会议记录持久化查询。
- 录制与回放。
- StreamForge Go 服务自研 SFU。
- StreamForge Redis 媒体实例路由。
- 自动故障迁移和通话恢复策略。
- Helm Chart、灰度发布、自动扩缩容策略。
- 虚拟背景、移动原生客户端、桌面客户端。

## 12. 验收标准

MVP 完成时必须满足：

- 用户可以注册并登录。
- 用户可以创建房间并获得房间号。
- 创建和加入房间时，Go 房间服务可以返回有效 `livekitUrl`、`livekitToken` 和 `livekitIdentity`。
- 至少两个浏览器标签页可以使用不同用户加入同一 LiveKit 房间。
- 同一房间用户可以互相看到视频、听到音频。
- 同一房间用户可以发送和接收聊天消息。
- 用户可以静音、关闭摄像头、恢复媒体轨道。
- 用户可以共享屏幕并停止共享。
- Docker Compose 可以启动完整本地环境。
- Kubernetes Manifest 可以部署前端、房间服务、用户服务、LiveKit、MySQL 和 Redis。
