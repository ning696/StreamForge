// Package room 实现"房间 + 房间内成员"的进程内实时状态管理。
//
// 【和 state 包的分工】
//   - state 包保存"房间存在""房间元信息"这类需要跨请求持久化的数据（Redis）。
//   - room 包只保存"当前进程里有哪些活着的 WebSocket 连接"、他们在哪个房间。
//     进程一重启这些数据全没了，也不该保留——反正客户端会重连。
//
// 【核心类型概览】
//   Hub    -> 全局中心，管所有房间
//   Room   -> 单个房间，管房间里的成员
//   Peer   -> 单个成员（对应一个 WebSocket 连接）
package room

import (
	"sync"
	"time"
)

// PeerState 是一个成员对外可见的"状态视图"，用于序列化成 JSON 广播给别人。
//
// 【为什么和 Peer 分开】
// Peer 内部还持有 send 函数、mutex 之类不能/不应发到客户端的字段；
// 广播时我们只挑出可以公开的信息，装进 PeerState。
type PeerState struct {
	PeerID          string `json:"peerId"`                    // 房间内唯一标识，由服务端生成
	UserID          int64  `json:"userId"`                    // 业务用户 ID
	Username        string `json:"username"`                  // 昵称
	LiveKitIdentity string `json:"livekitIdentity,omitempty"` // 该成员在 LiveKit 那边的 identity
	AudioEnabled    bool   `json:"audioEnabled"`              // 麦克风是否开着
	VideoEnabled    bool   `json:"videoEnabled"`              // 摄像头是否开着
	ScreenSharing   bool   `json:"screenSharing"`             // 是否在屏幕共享
}

// Snapshot 是"房间当前所有成员"的快照。
// 新成员进来时会先收到一份 Snapshot，这样它就知道谁已经在房间里了。
type Snapshot struct {
	Peers []PeerState `json:"peers"`
}

// ChatMessage 是聊天消息在房间内广播时的载荷。
// 【小知识】它的字段和 signaling.Message 里的部分字段有点重叠，
// 但在广播时会被塞进 signaling.Message.Payload，各司其职。
type ChatMessage struct {
	MessageID string `json:"messageId"`
	RoomID    string `json:"roomId"`
	PeerID    string `json:"peerId"`
	UserID    int64  `json:"userId"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	SentAt    int64  `json:"sentAt"`
}

// Peer 是"一个成员"的运行时对象。
//
// 【为什么带一把锁】
// - 主 goroutine 从 WebSocket 读消息、写 send 函数；
// - 别的 goroutine（广播）会调用 Send() 往这个 peer 发消息。
// 两者可能并发，所以要用 mu 保护 send / outbox 这两个字段。
type Peer struct {
	mu              sync.Mutex
	PeerID          string
	UserID          int64
	Username        string
	LiveKitIdentity string
	AudioEnabled    bool
	VideoEnabled    bool
	ScreenSharing   bool
	JoinedAt        time.Time

	// send 是"把消息真正写到 WebSocket 的函数"。
	// 【为什么用函数而不是直接持有 *websocket.Conn】
	// 让 room 包不依赖 websocket 库，保持解耦。
	// gateway 层建立连接后，通过 SetSender 把"怎么发"这个能力注入进来。
	send func(interface{})

	// outbox 是"send 还没被注入时的临时缓存"。
	// 用于极端顺序问题：如果广播先到、SetSender 后到，我们不能把消息丢掉。
	// 目前主要给单元测试用（测试里不真发到网络，就靠 Outbox() 检查发送内容）。
	outbox []interface{}
}

// NewPeer 构造一个 Peer。默认开麦、开摄像头，加入时间取当前。
func NewPeer(peerID string, userID int64, username string, livekitIdentity string) *Peer {
	return &Peer{
		PeerID:          peerID,
		UserID:          userID,
		Username:        username,
		LiveKitIdentity: livekitIdentity,
		AudioEnabled:    true,
		VideoEnabled:    true,
		JoinedAt:        time.Now(),
	}
}

// SetSender 把"如何往这个成员发消息"的能力注入到 Peer 里。
// 由 gateway 建立 WebSocket 后调用一次。
func (p *Peer) SetSender(send func(interface{})) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.send = send
}

// Send 给这个成员发一条消息。
//
// 【逻辑】
//   - 已经有 send 函数：直接调用它（真正走 WebSocket）
//   - 还没有 send：先缓存到 outbox，避免消息丢失（主要给测试打桩使用）
func (p *Peer) Send(message interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.send != nil {
		p.send(message)
		return
	}
	p.outbox = append(p.outbox, message)
}

// Outbox 返回目前缓存里的消息副本。仅测试用。
//
// 【为什么要返回副本而不是原 slice】
// 直接返回 p.outbox 会把内部状态泄漏出去——外面追加/修改，Peer 内部也跟着变。
// append([]interface{}(nil), p.outbox...) 这一行是 Go 里"复制一个 slice"的惯用写法。
func (p *Peer) Outbox() []interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]interface{}(nil), p.outbox...)
}

// State 把 Peer 转成对外可见的 PeerState，用于广播。
func (p *Peer) State() PeerState {
	return PeerState{
		PeerID:          p.PeerID,
		UserID:          p.UserID,
		Username:        p.Username,
		LiveKitIdentity: p.LiveKitIdentity,
		AudioEnabled:    p.AudioEnabled,
		VideoEnabled:    p.VideoEnabled,
		ScreenSharing:   p.ScreenSharing,
	}
}

// Room 表示一个房间的实时状态，包括房间内所有活着的成员。
type Room struct {
	mu              sync.Mutex        // 保护下面的 peers map
	RoomID          string
	CreatedByUserID int64
	CreatedAt       time.Time
	peers           map[string]*Peer // peerID -> Peer
}

// NewRoom 构造一个 Room，peers 用 make 初始化避免 nil map panic。
func NewRoom(roomID string, createdByUserID int64) *Room {
	return &Room{
		RoomID:          roomID,
		CreatedByUserID: createdByUserID,
		CreatedAt:       time.Now(),
		peers:           make(map[string]*Peer),
	}
}

// AddPeer 往房间里加一个成员。
func (r *Room) AddPeer(peer *Peer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[peer.PeerID] = peer
}

// RemovePeer 从房间里移除一个成员。
//
// 返回值：
//   *Peer —— 被移除的成员对象（若不存在则为 nil）
//   bool  —— 是否真的存在并被移除了
//   int   —— 移除后房间还剩多少人（调用方可以据此判断"要不要销毁房间"）
//
// 【多返回值】Go 允许函数返回任意多个值，是很地道的错误+数据组合方式。
func (r *Room) RemovePeer(peerID string) (*Peer, bool, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	peer, ok := r.peers[peerID]
	delete(r.peers, peerID) // delete 对不存在的 key 也不会 panic，所以放心用
	return peer, ok, len(r.peers)
}

// Snapshot 拍一份"当前房间所有成员的状态"快照。
func (r *Room) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 预分配 slice 容量，避免多次扩容。
	peers := make([]PeerState, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer.State())
	}
	return Snapshot{Peers: peers}
}

// RecordChat 记录并广播一条聊天消息。
// 目前只是"广播"，未来可以在这里加"入库/存历史"的逻辑。
func (r *Room) RecordChat(message ChatMessage) {
	r.Broadcast(message)
}

// Broadcast 把消息发给房间里"所有"成员。
//
// 【关键并发技巧】
// 不能"边持锁边调用 peer.Send"：Peer.Send 内部也会加锁，
// 万一 send 函数触发别的路径反过来锁 room，就会死锁。
// 所以先在锁里把 peers 拷成一个本地 slice，锁外再遍历发送。
func (r *Room) Broadcast(message interface{}) {
	r.mu.Lock()
	peers := make([]*Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer)
	}
	r.mu.Unlock()

	for _, peer := range peers {
		peer.Send(message)
	}
}

// BroadcastExcept 把消息发给"除了 peerID 之外"的所有成员。
// 典型场景：告诉大家"新人来了"，但不用告诉新人自己。
func (r *Room) BroadcastExcept(peerID string, message interface{}) {
	r.mu.Lock()
	peers := make([]*Peer, 0, len(r.peers))
	for id, peer := range r.peers {
		if id != peerID {
			peers = append(peers, peer)
		}
	}
	r.mu.Unlock()

	for _, peer := range peers {
		peer.Send(message)
	}
}

// Hub 是"整个进程里所有房间"的中心索引。
// 一个 media-service 实例只有一个 Hub。
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

// NewHub 构造一个 Hub。
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

// EnsureRoom "保证房间存在"：
//   - 已经存在就返回已有的 Room
//   - 不存在就创建一个新的 Room 并返回
//
// 【为什么叫 Ensure 而不是 Get/Create】
// gateway 里两种入口都会走到这里：创建房间 API 和 WebSocket join。
// 前者会创建新的，后者往往复用已有的。用一个 Ensure 把逻辑收敛在一起，
// 而且是"原子操作"（在锁内完成"查 + 建"），避免并发下两个 goroutine 各建一个的问题。
func (h *Hub) EnsureRoom(roomID string, createdByUserID int64) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.rooms[roomID]; ok {
		return existing
	}
	created := NewRoom(roomID, createdByUserID)
	h.rooms[roomID] = created
	return created
}

// GetRoom 查一个房间，找不到返回 (nil, false)。
func (h *Hub) GetRoom(roomID string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	roomState, ok := h.rooms[roomID]
	return roomState, ok
}

// DeleteRoom 从 Hub 里移除一个房间（通常在房间没人时调用）。
func (h *Hub) DeleteRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, roomID)
}

// Stats 返回 (房间总数, 成员总数)，主要给 /health 端点用。
//
// 【为什么不在锁内直接累加】
// 计算成员数要调用 room.Snapshot()，而 Snapshot 内部也要加锁。
// 若这里持着 Hub 的锁去等每个 Room 的锁，理论上有死锁风险。
// 所以先拿出 rooms 副本、释放 Hub 锁，再单独去 Snapshot 每个房间。
func (h *Hub) Stats() (int, int) {
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, roomState := range h.rooms {
		rooms = append(rooms, roomState)
	}
	h.mu.Unlock()

	peerCount := 0
	for _, roomState := range rooms {
		peerCount += len(roomState.Snapshot().Peers)
	}
	return len(rooms), peerCount
}
