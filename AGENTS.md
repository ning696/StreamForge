# StreamForge Codex 协作规则

## Local Toolchain

- Go SDK is fixed at `C:\Users\admin\sdk\go1.25.6`.
- Codex must run Go commands through the absolute binary path instead of the `go` executable from PATH:

```powershell
& 'C:\Users\admin\sdk\go1.25.6\bin\go.exe' version
& 'C:\Users\admin\sdk\go1.25.6\bin\go.exe' test ./...
& 'C:\Users\admin\sdk\go1.25.6\bin\go.exe' mod tidy
```

- Do not run bare `go ...`; it can resolve to `C:\Program Files\Go\bin\go.exe` from PATH.

## 项目定位

StreamForge 是一个基于 Web 的实时音视频会议系统。`livekit-standalone-cluster` 分支的 MVP 阶段采用多服务架构：

- 前端：Vue 3 + TypeScript + Element Plus + `livekit-client`
- 房间服务：Go + REST + WebSocket，负责房间创建、加入、LiveKit Token/Identity 签发和业务实时消息
- 媒体服务：LiveKit 独立集群，负责 WebRTC 信令、SFU 转发、媒体轨道、ICE、屏幕共享和媒体状态
- 用户服务：Java Spring Boot，提供简单用户体系
- 数据存储：MySQL 存储用户账号数据
- 运行时协调：LiveKit 集群使用 Redis 做节点协调；StreamForge 可使用 Redis 保存可过期的房间摘要和临时在线状态
- 部署：Docker Compose + Kubernetes Manifest

MVP 的核心约束是：StreamForge 不自研 SFU，不在 Go 服务中创建或保存 WebRTC PeerConnection、SDP、ICE Candidate、Track 等媒体对象；这些能力全部由 LiveKit 独立集群承担。StreamForge 负责业务房间、用户身份透传、LiveKit Token/Identity 签发、聊天和部署集成。

## 文档读取规则

不要在每次任务中读取整个 `doc/` 目录。按任务类型读取必要文档。

开始任何开发任务前，必须先阅读：

- `doc/00-项目规划.md`
- `doc/01-MVP需求规格.md`
- `doc/02-系统架构设计.md`

如果修改 WebSocket、信令、房间创建、加入、离开、聊天、在线状态，额外阅读：

- `doc/03-WebSocket信令协议.md`

如果修改 WebRTC、SFU、媒体轨道、屏幕共享、ICE、SDP、PeerConnection，额外阅读：

- `doc/04-WebRTC-SFU媒体设计.md`

如果修改前端页面、组件、路由、Pinia 状态、Element Plus UI，额外阅读：

- `doc/05-前端设计.md`

如果修改 MySQL、Redis、实体模型、内存状态、ID 规则，额外阅读：

- `doc/06-数据与状态模型.md`

如果修改测试、验收、质量门禁，额外阅读：

- `doc/07-测试与验收方案.md`

如果修改 Docker、docker-compose、Kubernetes、环境变量、端口、部署脚本，额外阅读：

- `doc/08-部署与本地运行手册.md`

如果任务涉及技术决策背景，额外阅读相关 ADR：

- `doc/adr/0001-选择SFU而非P2P.md`
- `doc/adr/0002-MVP采用多实例媒体服务与房间级路由.md`
- `doc/adr/0003-前端UI组件库选型.md`
- `doc/adr/0004-MVP采用LiveKit独立集群承载SFU.md`

## 实现边界

- 不把 MVP 写成 P2P 音视频方案。
- 不在 MVP 中实现 JWT、Session、OAuth2 等完整鉴权。
- 不在 MVP 中实现房间历史、聊天记录、会议记录的持久化查询。
- 不在 StreamForge Go 服务中实现 SDP/ICE/Track 转发或自研 SFU。
- 不把 WebRTC 连接、PeerConnection、Track 对象存入 Redis 或 MySQL。
- Redis 只保存 LiveKit 协调数据、业务房间摘要、临时在线状态等可序列化运行时数据。
- MySQL 只保存需要持久化的业务数据，MVP 阶段主要是用户账号数据。
- LiveKit access token 只作为 LiveKit 房间准入凭证，不代表 StreamForge 已实现完整鉴权体系。

## 代码组织原则

后续创建代码时，优先保持小文件和清晰边界。

Go 房间服务建议按职责拆分：

- `gateway`：HTTP / WebSocket 入口、请求上下文、响应封装
- `room`：业务房间创建、加入、离开、房间摘要
- `livekit`：LiveKit 配置、Token 签发、房间名和参与者身份映射
- `signaling`：WebSocket 业务消息解析、校验和分发
- `chat`：房间内文字消息广播
- `state`：Redis 中的可过期房间摘要、临时在线状态
- `config`：环境变量、LiveKit、Redis 配置
- `log`：结构化日志封装

Java 用户服务建议按职责拆分：

- `controller`：REST API
- `service`：用户注册、登录校验、用户查询
- `repository`：MySQL 持久化访问
- `entity` / `model`：用户实体和请求响应模型

前端建议按职责拆分：

- `views`：页面级组件
- `components`：可复用 UI 组件
- `stores`：Pinia 状态
- `api`：REST API 调用
- `livekit`：LiveKit 客户端封装
- `signaling`：WebSocket 客户端封装，用于聊天或业务消息

## 文档维护规则

当实现改变以下内容时，必须同步更新对应文档：

- MVP 范围变化：更新 `doc/00-项目规划.md` 和 `doc/01-MVP需求规格.md`
- 服务边界变化：更新 `doc/02-系统架构设计.md`
- 信令消息变化：更新 `doc/03-WebSocket信令协议.md`
- 媒体链路变化：更新 `doc/04-WebRTC-SFU媒体设计.md`
- 前端结构变化：更新 `doc/05-前端设计.md`
- 数据结构变化：更新 `doc/06-数据与状态模型.md`
- 测试策略变化：更新 `doc/07-测试与验收方案.md`
- 部署方式变化：更新 `doc/08-部署与本地运行手册.md`

## 质量要求

- 新增业务逻辑应配套测试。
- 修改协议、数据结构、部署配置时，必须说明兼容性影响。
- 音视频相关改动必须手工验证至少两个浏览器标签页加入同一房间。
- LiveKit 集群相关改动必须验证前端可通过后端签发的 Token 加入同一个 LiveKit 房间。
- 前端改动必须保证主要界面在桌面宽屏和移动窄屏下不出现明显遮挡或溢出。
