# ADR 0004：MVP 采用 LiveKit 独立集群承载 SFU

## 状态

Accepted

## 背景

StreamForge 的目标是交付一个可运行的 Web 多人音视频会议 MVP。早期方案计划由 Go 媒体服务直接基于 `pion/webrtc` 实现 WebRTC 信令、ICE、PeerConnection、Track 发布订阅和 SFU 转发，并通过 Redis 保存 `roomId -> mediaInstanceId` 的房间级路由。

在推进阶段二时，项目选择接入成熟 SFU 框架，以降低 WebRTC/SFU 底层实现风险，并把开发重点放到业务房间、用户体系、前端体验、部署集成和后续产品能力上。

## 决策

MVP 采用 **LiveKit 独立集群** 承载 WebRTC 与 SFU 能力。

StreamForge Go 服务不再自研 SFU，不再处理 SDP Offer/Answer、ICE Candidate、PeerConnection、RTP/RTCP 和 Track 转发。Go 服务保留为房间入口和业务实时服务，负责：

- 创建和加入业务房间。
- 生成 LiveKit room name。
- 根据登录用户信息签发 LiveKit access token。
- 返回 `livekitUrl` 和 `livekitToken` 给前端。
- 提供房间内文字聊天等业务实时消息，或后续迁移到 LiveKit data packet。
- 保存可过期的房间摘要和调试状态。

LiveKit 独立集群负责：

- WebRTC 信令。
- ICE / NAT 穿越。
- SFU 媒体转发。
- 音频、摄像头视频、屏幕共享 Track 发布订阅。
- 参与者连接状态和媒体状态。
- 多节点媒体调度和集群协调。

## 架构关系

```text
Vue 前端
  ├─ REST ─────────────▶ Java 用户服务 ─────▶ MySQL
  ├─ REST ─────────────▶ Go 房间服务 ───────▶ Redis 可过期业务状态
  ├─ WebSocket 可选 ───▶ Go 房间服务        聊天/业务消息
  └─ LiveKit SDK ──────▶ LiveKit 独立集群 ─▶ Redis 集群协调
```

前端通过 StreamForge REST 创建或加入房间，拿到 `livekitUrl` 和 `livekitToken` 后，使用 `livekit-client` 连接 LiveKit 房间并发布/订阅媒体。

## 为什么不继续自研 Go SFU

自研 SFU 需要完整处理：

- SDP Offer/Answer 协商。
- ICE Candidate 缓存、交换和连接状态。
- PeerConnection 生命周期。
- Track 发布、订阅、转发和清理。
- RTP/RTCP 转发、拥塞反馈和网络异常。
- 多浏览器兼容问题。
- 多实例媒体调度、端口暴露和故障处理。

这些能力超出 MVP 的主要产品风险。LiveKit 已提供成熟的服务端、前端 SDK、部署模式和集群能力，更适合作为 MVP 的媒体底座。

## 影响

正面影响：

- 降低 WebRTC/SFU 底层实现复杂度。
- 前端通过 LiveKit SDK 处理媒体发布、订阅、屏幕共享、设备控制和参与者状态。
- Go 服务职责更清晰，聚焦业务房间、Token 签发、聊天和集成。
- 部署可以直接利用 LiveKit 的 Docker/Kubernetes 方案。

负面影响：

- 项目不再展示“从零实现 Go SFU”的能力。
- MVP 引入 LiveKit API key/secret 和 token 签发逻辑。
- 部署需要理解 LiveKit 的 RTC 端口、TURN、Redis 和公网 IP 配置。
- StreamForge 的 Redis 不再是媒体实例路由事实源，相关文档和代码需要迁移。

## 约束

- MVP 仍采用 SFU，不回退到 P2P 或 MCU。
- MVP 不在 StreamForge Go 服务中创建或保存 WebRTC 对象。
- LiveKit access token 只作为 LiveKit 房间准入凭证，不代表 StreamForge 已实现完整 JWT/Session/OAuth2 鉴权体系。
- MySQL 仍只保存用户账号等持久化业务数据。
- Redis 中只能保存可序列化、可过期的运行时协调数据，不能保存媒体对象。

## 被替代决策

本 ADR 替代 `ADR 0002：MVP 采用多实例媒体服务与房间级路由`。旧方案保留为历史记录，不再作为本分支实现依据。

## 相关文档

- `doc/00-项目规划.md`
- `doc/01-MVP需求规格.md`
- `doc/02-系统架构设计.md`
- `doc/03-WebSocket信令协议.md`
- `doc/04-WebRTC-SFU媒体设计.md`
- `doc/05-前端设计.md`
- `doc/06-数据与状态模型.md`
- `doc/08-部署与本地运行手册.md`
