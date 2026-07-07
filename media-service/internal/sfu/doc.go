// Package sfu 是 SFU（Selective Forwarding Unit，选择性转发单元）相关代码的占位包。
//
// 【背景知识】
// SFU 是一种音视频转发架构：客户端只上传一份自己的媒体流到服务器，
// 服务器再把这份流"选择性地"转发给房间里的其他人。
// 相比 P2P（客户端两两直连）它更省客户端带宽，相比 MCU（服务端混流）它更省服务端算力。
//
// 【当前状态】
// 本项目使用 LiveKit 作为外部 SFU 服务器，因此这个包目前只是一个占位（placeholder），
// 用来在未来放置和 SFU 直接交互的封装代码。真正的 SFU 逻辑由 LiveKit 集群承担，
// 我们只是通过 livekit 子包生成访问 Token，让客户端能连上 LiveKit。
package sfu
