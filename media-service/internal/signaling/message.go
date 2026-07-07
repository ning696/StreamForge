// Package signaling 定义客户端 <-> 服务端之间通过 WebSocket 传输的"信令消息"格式。
//
// 【什么是"信令"？】
// 在音视频/实时通信里，信令（Signaling）指的是"控制类消息"——加入房间、离开房间、
// 有人加入了、心跳、聊天……它不是媒体数据（音视频码流走 LiveKit/WebRTC），
// 而是围绕房间生命周期和成员状态的元信息。
//
// 【为什么单独一个包】
// 服务端（gateway）和测试代码都要用到这些类型和常量。放在共享包里让双方引用同一份定义，
// 避免"客户端认为叫 join，服务端叫 room_join"这样的对不上号问题。
package signaling

import (
	"encoding/json" // Go 标准库：JSON 编解码
	"errors"        // 生成 error 值
)

// 消息类型常量。这些字符串会作为 JSON 里 "type" 字段的值出现，
// 客户端和服务端都根据它来分派到对应的处理逻辑。
// 【小知识】const (...) 是分组常量声明，等价于分别写多个 const。
// 用常量比每次写字符串字面量更安全：拼错时编译不过。
const (
	TypeRoomJoin     = "room.join"     // 客户端 -> 服务端：请求加入房间
	TypeRoomJoined   = "room.joined"   // 服务端 -> 客户端：加入成功回执
	TypeRoomSnapshot = "room.snapshot" // 服务端 -> 客户端：当前房间成员快照（进入房间时下发一份）
	TypeRoomLeave    = "room.leave"    // 客户端 -> 服务端：主动离开房间
	TypePeerJoined   = "peer.joined"   // 服务端 -> 所有客户端（广播）：某个新成员进来了
	TypePeerLeft     = "peer.left"     // 服务端 -> 所有客户端（广播）：某个成员离开了
	TypeChatSend     = "chat.send"     // 客户端 -> 服务端：发送一条聊天
	TypeChatMessage  = "chat.message"  // 服务端 -> 所有客户端（广播）：分发一条聊天
	TypePing         = "ping"          // 客户端 -> 服务端：心跳，探测连接是否活着
	TypePong         = "pong"          // 服务端 -> 客户端：心跳回应
	TypeError        = "error"         // 服务端 -> 客户端：错误消息
)

// ErrInvalidMessage 表示收到的消息在结构上是"不合法"的（比如缺 type 或 payload）。
var ErrInvalidMessage = errors.New("invalid signaling message")

// Message 是所有信令消息的统一外层结构。
//
// 【设计思路】所有信令共享同一个外壳，type 字段告诉接收方应该怎么解析 payload。
// 这样 WebSocket 传输层只处理一种结构，具体业务再按 type 分派——非常常见的做法。
//
// 【结构体 tag 的作用】反引号里的 `json:"xxx"` 告诉 encoding/json 包：
//   - 序列化时 Go 字段名要变成什么 JSON key
//   - "omitempty" 表示字段为零值时（例如空字符串、0），在 JSON 里省略掉
type Message struct {
	Type      string          `json:"type"`                // 消息类型，见上面的常量
	RequestID string          `json:"requestId,omitempty"` // 客户端可选填，用来把"请求"和"回执"配对
	RoomID    string          `json:"roomId"`              // 所属房间 ID
	PeerID    string          `json:"peerId,omitempty"`    // 房间内的成员 ID（由服务端生成，进入房间前为空）
	UserID    int64           `json:"userId"`              // 业务系统里的用户 ID
	Username  string          `json:"username"`            // 显示昵称
	Timestamp int64           `json:"timestamp,omitempty"` // 服务端时间戳（毫秒），可选
	// Payload 是"具体消息内容"，不同 type 对应不同结构。
	// 用 json.RawMessage（本质是 []byte）表示"先不解析，等看到 type 再决定怎么解"，
	// 这是处理"多态 JSON"的经典技巧，避免为每种 type 定义完整结构体。
	Payload json.RawMessage `json:"payload"`
}

// Decode 把 WebSocket 原始字节反序列化成 Message。
//
// 参数：data —— WebSocket 收到的一帧 JSON 字节流
// 返回：解析出的 Message；若 JSON 无效或缺少必要字段则返回 error。
//
// 【为什么要额外检查 Type 和 Payload】
// json.Unmarshal 只保证语法上是合法 JSON，不校验"必须有 type 字段"。
// 我们要求任何合法消息必须有 type 和 payload，否则后续 handler 拿不到任何有用信息，
// 干脆在入口处拦下来。
func Decode(data []byte) (Message, error) {
	var message Message
	// & 取地址：Unmarshal 需要接收方的指针，因为它要修改传进来的结构体。
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, err
	}
	if message.Type == "" || len(message.Payload) == 0 {
		return Message{}, ErrInvalidMessage
	}
	return message, nil
}

// Payload 把任意 Go 值序列化成 json.RawMessage，方便塞进 Message.Payload。
//
// 【设计权衡】
// 这里对错误做了容忍处理——如果 Marshal 失败（几乎不可能，除非传了循环引用），
// 就退化为一个空对象 {} 而不是 panic 或返回 error。
// 因为它常在业务代码里链式使用（例如 signaling.Payload(map[...]{...}) 直接当参数），
// 出错时给一个"能被安全发送出去"的最小值，比中断整个流程更实用。
//
// 参数：value —— 任何可以被 JSON 序列化的值（struct / map / slice / 基本类型）
// 【interface{}】在 Go 里表示"任意类型"（新版本也叫 any）。
func Payload(value interface{}) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
