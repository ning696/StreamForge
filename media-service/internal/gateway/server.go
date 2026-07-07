// Package gateway 是 media-service 的"HTTP/WebSocket 网关层"。
//
// 【本包定位】
//   - 对外：暴露 REST API（创建房间、加入房间）和 WebSocket（实时信令）。
//   - 对内：把请求路由到 state（房间元数据）、livekit（Token）、room（成员管理）、
//           chat（聊天校验）、signaling（消息协议）这几个下层包，串起来完成一个业务动作。
//
// 一个典型的调用链：
//   前端 POST /api/rooms
//     -> gateway.handleRooms
//     -> state.Store.CreateRoom  (Redis 持久化)
//     -> room.Hub.EnsureRoom     (内存注册)
//     -> livekit.Issuer.Issue    (签发 Token)
//     -> 返回前端 JSON
//
//   前端 WS /ws/rooms/{id}
//     -> gateway.handleWebSocket -> wsSession.run -> handleJoin/handleChat/...
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand" // 生成随机 6 位房间号
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	// gorilla/websocket 是 Go 里最常用的 WebSocket 库
	"github.com/gorilla/websocket"

	"streamforge/media-service/internal/chat"
	"streamforge/media-service/internal/config"
	livekittoken "streamforge/media-service/internal/livekit"
	"streamforge/media-service/internal/room"
	"streamforge/media-service/internal/signaling"
	"streamforge/media-service/internal/state"
)

// Server 是 gateway 的核心结构，聚合了它需要的所有依赖。
//
// 【依赖注入】gateway 不自己创建 store、issuer、hub，而是从 main.go 通过 NewServer
// 传入。这样在测试里可以塞入 MemoryStore、假 Issuer 等。
type Server struct {
	cfg      config.Config
	store    state.Store          // 房间元数据存储（Redis 或 Memory）
	issuer   *livekittoken.Issuer // LiveKit Token 签发器
	hub      *room.Hub            // 内存中的房间/成员中心
	upgrader websocket.Upgrader   // HTTP -> WebSocket 的升级器
}

// RoomRequest 是 POST /api/rooms 和 /api/rooms/{id}/join 的请求体。
type RoomRequest struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
}

// RoomResponse 是上面两个 API 的返回体。
// 前端拿到这份数据后就能连上 LiveKit 和我们的 WebSocket 了。
type RoomResponse struct {
	RoomID          string `json:"roomId"`          // 房间 ID
	LiveKitRoomName string `json:"livekitRoomName"` // 对应的 LiveKit 房间名
	LiveKitURL      string `json:"livekitUrl"`      // LiveKit 服务地址
	LiveKitToken    string `json:"livekitToken"`    // JWT Token（凭它才能连 LiveKit）
	LiveKitIdentity string `json:"livekitIdentity"` // 本次连接在 LiveKit 侧的 identity
	AppWSURL        string `json:"appWsUrl"`        // 我们自己的信令 WebSocket 地址
}

// ErrorResponse 统一的错误响应结构。
type ErrorResponse struct {
	Code    string `json:"code"`    // 便于前端做逻辑判断的错误码（如 "ROOM_NOT_FOUND"）
	Message string `json:"message"` // 给人看的错误信息
}

// NewServer 是 Server 的构造函数。
func NewServer(cfg config.Config, store state.Store, issuer *livekittoken.Issuer, hub *room.Hub) *Server {
	return &Server{
		cfg:    cfg,
		store:  store,
		issuer: issuer,
		hub:    hub,
		upgrader: websocket.Upgrader{
			// 【CheckOrigin】gorilla 默认会做同源检查，禁止跨域 WebSocket。
			// 我们的场景（前后端分离 + 开发环境）需要允许跨域，所以直接 return true。
			// 【安全提示】生产环境应该改成"只允许我们的前端域名"，避免任意站点搭一个页面就来连我们的 WS。
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Handler 组装出整个 gateway 的 http.Handler，返回给 main.go 挂载到 http.Server。
//
// 【mux 是什么】http.ServeMux 是 Go 标准库自带的最小路由器，
// 按注册的 URL 前缀匹配。够我们用就没引第三方路由库。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)                // 健康检查
	mux.HandleFunc("/api/rooms", s.handleRooms)              // POST 创建房间
	mux.HandleFunc("/api/rooms/", s.handleRoomAction)        // POST /api/rooms/{id}/join
	mux.HandleFunc("/ws/rooms/", s.handleWebSocket)          // WS  /ws/rooms/{id}
	return withCORS(mux)                                     // 最外层套一个 CORS 中间件
}

// handleHealth 处理 GET /health。
//
// 用途：K8s / 负载均衡的存活探测；运维检查依赖是否配好。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	rooms, peers := s.hub.Stats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "UP",
		"livekitConfigured": s.issuer != nil && s.issuer.Configured(),
		"rooms":             rooms,
		"appPeers":          peers,
		"redis":             "UP",
	})
}

// handleRooms 处理 POST /api/rooms —— 创建新房间。
func (s *Server) handleRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	// 1) 解请求体，校验 userId/username。校验失败 decodeRoomRequest 内部已经写好 400 响应。
	request, ok := decodeRoomRequest(w, r)
	if !ok {
		return
	}

	// 2) 生成一个未被占用的 6 位房间号。
	roomID, err := s.generateRoomID(r.Context())
	if err != nil {
		writeStateError(w, err)
		return
	}

	// 3) 组装房间元信息并写入 Redis。
	meta := state.RoomMeta{
		RoomID:          roomID,
		LiveKitRoomName: livekittoken.RoomName(roomID),
		CreatedByUserID: request.UserID,
		CreatedAt:       time.Now(),
		Status:          "active",
	}
	if err := s.store.CreateRoom(r.Context(), meta); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	// 4) 在内存 Hub 里把这个房间创建出来。EnsureRoom 是幂等的。
	s.hub.EnsureRoom(roomID, request.UserID)

	// 5) 签发 Token 并返回给前端。
	response, err := s.roomResponse(meta, request)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "LIVEKIT_UNAVAILABLE", "livekit is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleRoomAction 处理 POST /api/rooms/{id}/join —— 加入已有房间。
//
// 【和 handleRooms 的区别】创建时 roomID 是我们生成的；加入时 roomID 是前端传的，
// 我们要先从存储里查它是否存在。
func (s *Server) handleRoomAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	// 从 URL "/api/rooms/{id}/join" 里抠出 id。
	roomID, ok := parseJoinPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	request, ok := decodeRoomRequest(w, r)
	if !ok {
		return
	}
	// 查房间存在与否；不存在则 404。
	meta, err := s.store.GetRoom(r.Context(), roomID)
	if err != nil {
		writeStateError(w, err)
		return
	}
	// 房间元数据存在，但当前进程内存里可能还没建 Room（例如另一实例创建的、这个实例第一次接触），
	// 用 EnsureRoom 兜底建一个。0 表示"我不是创建者"，仅占位。
	s.hub.EnsureRoom(roomID, 0)

	response, err := s.roomResponse(meta, request)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "LIVEKIT_UNAVAILABLE", "livekit is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleWebSocket 处理 GET /ws/rooms/{id} —— 建立 WebSocket 长连接。
//
// HTTP 到 WebSocket 的转换叫"协议升级（Upgrade）"，Upgrader.Upgrade 会替我们处理握手。
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimPrefix(r.URL.Path, "/ws/rooms/")
	if roomID == "" {
		writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "room not found")
		return
	}
	// 建 WS 之前先确认房间元数据在 store 里存在。
	meta, err := s.store.GetRoom(r.Context(), roomID)
	if err != nil {
		writeStateError(w, err)
		return
	}
	// 升级为 WebSocket 连接。失败时 gorilla 已经写好响应，我们直接返回。
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	// 每个 WS 连接创建一个 session 对象，专门管这条连接的生命周期。
	session := &wsSession{server: s, conn: conn, roomID: roomID, meta: meta}
	session.run(r.Context())
}

// roomResponse 组装出给前端的 RoomResponse（含签发 Token 的步骤）。
func (s *Server) roomResponse(meta state.RoomMeta, request RoomRequest) (RoomResponse, error) {
	issued, err := s.issuer.Issue(livekittoken.TokenRequest{
		RoomID:   meta.RoomID,
		RoomName: meta.LiveKitRoomName,
		UserID:   request.UserID,
		Username: request.Username,
	})
	if err != nil {
		return RoomResponse{}, err
	}
	return RoomResponse{
		RoomID:          meta.RoomID,
		LiveKitRoomName: meta.LiveKitRoomName,
		LiveKitURL:      issued.URL,
		LiveKitToken:    issued.Token,
		LiveKitIdentity: issued.Identity,
		// 把公网 base URL 和路径拼起来，前端只需要用这个字符串直接 new WebSocket(url) 就能连。
		AppWSURL: strings.TrimRight(s.cfg.PublicWSBaseURL, "/") + "/ws/rooms/" + meta.RoomID,
	}, nil
}

// generateRoomID 生成一个"未被占用"的 6 位数字房间号。
//
// 【思路】随机 100000~999999 之间的一个数，去 store 里查在不在，
// 不在就用它；重试最多 10 次防止死循环。
// 【为什么用 6 位数字】方便口播——"我的房间号是 123456"比一串 UUID 好念太多。
func (s *Server) generateRoomID(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		roomID := strconv.Itoa(100000 + rand.Intn(900000))
		// 用 GetRoom 是否返回 ErrRoomNotFound 来判断"空号"。
		// errors.Is 会正确处理包装过的错误，比直接 == 更安全。
		if _, err := s.store.GetRoom(ctx, roomID); errors.Is(err, state.ErrRoomNotFound) {
			return roomID, nil
		}
	}
	return "", errors.New("generate room id failed")
}

// wsSession 表示"一条 WebSocket 连接的会话"。
//
// 每个 WebSocket 连接对应一个 session；session 里持有：
//   - conn：这条底层连接
//   - peer：该连接对应的房间成员（在收到 room.join 后才有）
//   - roomID/meta：目标房间
type wsSession struct {
	server *Server
	conn   *websocket.Conn
	roomID string
	meta   state.RoomMeta
	peer   *room.Peer
	// mu 保护 conn.WriteJSON —— gorilla/websocket 的 Conn 不允许多个 goroutine 同时写。
	mu sync.Mutex
}

// run 是这条 WS 连接的主循环：读 -> 分派 -> 直到断开。
func (s *wsSession) run(ctx context.Context) {
	// defer 保证函数返回时（不管什么原因）都会走 close 清理。
	defer s.close(ctx)

	for {
		// ReadMessage 阻塞等一条消息；对方关闭 / 网络断开会返回 error 并跳出循环。
		// 【返回值】第一个是消息类型（text/binary），我们不关心所以用 _ 丢掉。
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		message, err := signaling.Decode(data)
		if err != nil {
			s.sendError("", "BAD_REQUEST", "bad request")
			continue // 一条消息坏了不代表连接坏了，继续读下一条
		}
		s.handle(ctx, message)
	}
}

// handle 根据 message.Type 分派到具体的处理方法。
func (s *wsSession) handle(ctx context.Context, message signaling.Message) {
	switch message.Type {
	case signaling.TypeRoomJoin:
		s.handleJoin(message)
	case signaling.TypeRoomLeave:
		s.close(ctx) // 关掉连接（内部会做广播 + 清理）
	case signaling.TypeChatSend:
		s.handleChat(message)
	case signaling.TypePing:
		// 心跳：客户端发 ping，服务端立刻回一条 pong，证明连接活着。
		s.send(signaling.Message{
			Type:      signaling.TypePong,
			RequestID: message.RequestID,
			RoomID:    s.roomID,
			PeerID:    s.peerID(),
			UserID:    message.UserID,
			Username:  message.Username,
			Timestamp: unixMillis(time.Now()),
			Payload:   signaling.Payload(map[string]string{}),
		})
	default:
		s.sendError(message.RequestID, "BAD_REQUEST", "unsupported message type")
	}
}

// handleJoin 处理 room.join 消息：把当前 WS 连接注册成房间里的一个成员。
func (s *wsSession) handleJoin(message signaling.Message) {
	// 客户端可能同时带了 roomId；我们要求它必须和 URL 里的 roomID 一致，防止张冠李戴。
	if message.RoomID != "" && message.RoomID != s.roomID {
		s.sendError(message.RequestID, "BAD_REQUEST", "room id mismatch")
		return
	}
	// 从 payload 里取出 livekitIdentity（客户端可能想把 LiveKit 的 identity 同步过来）。
	// 【匿名 struct】Go 允许在这里当场定义一个"临时结构体"，只用一次不用起名字。
	var payload struct {
		LiveKitIdentity string `json:"livekitIdentity"`
	}
	// 忽略解析错误：即便 payload 结构不对，我们也允许"没有 identity"地加入。
	_ = json.Unmarshal(message.Payload, &payload)

	// 拿到房间对象（不存在就创建）。
	roomState := s.server.hub.EnsureRoom(s.roomID, message.UserID)

	// 生成一个进程内唯一的 peerID：前缀 + UUID 的前 8 位。
	peer := room.NewPeer("app-peer-"+uuid.NewString()[:8], message.UserID, message.Username, payload.LiveKitIdentity)
	// 把"如何发消息"这个能力注入进 peer。
	// 这里传的是一个闭包：调用 peer.Send 时最终走的是本 wsSession 的 send 方法。
	peer.SetSender(func(value interface{}) { s.send(value) })
	s.peer = peer

	// 把新 peer 加入房间。
	roomState.AddPeer(peer)

	// 【下面连发三条消息】
	// 1) room.joined：告诉当前客户端"你加入成功了"。
	s.send(signaling.Message{
		Type:      signaling.TypeRoomJoined,
		RequestID: message.RequestID,
		RoomID:    s.roomID,
		PeerID:    peer.PeerID,
		UserID:    peer.UserID,
		Username:  peer.Username,
		Timestamp: unixMillis(time.Now()),
		Payload: signaling.Payload(map[string]string{
			"livekitRoomName": s.meta.LiveKitRoomName,
			"livekitIdentity": peer.LiveKitIdentity,
		}),
	})
	// 2) room.snapshot：把当前房间里已有的成员列表给这位新客户端。
	s.send(signaling.Message{
		Type:      signaling.TypeRoomSnapshot,
		RoomID:    s.roomID,
		PeerID:    peer.PeerID,
		UserID:    peer.UserID,
		Username:  peer.Username,
		Timestamp: unixMillis(time.Now()),
		Payload:   signaling.Payload(roomState.Snapshot()),
	})
	// 3) peer.joined：广播给房间里"除了自己"的所有人，通知他们有新成员。
	roomState.BroadcastExcept(peer.PeerID, signaling.Message{
		Type:      signaling.TypePeerJoined,
		RoomID:    s.roomID,
		PeerID:    peer.PeerID,
		UserID:    peer.UserID,
		Username:  peer.Username,
		Timestamp: unixMillis(time.Now()),
		Payload:   signaling.Payload(peer.State()),
	})
}

// handleChat 处理 chat.send 消息：校验内容后广播成 chat.message。
func (s *wsSession) handleChat(message signaling.Message) {
	// 必须先 join 过（有 peer）才能发聊天。
	if s.peer == nil {
		s.sendError(message.RequestID, "PEER_NOT_JOINED", "peer is not joined")
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		s.sendError(message.RequestID, "BAD_REQUEST", "bad request")
		return
	}
	// 走 chat 包的统一校验（空 / 超长）。
	content, err := chat.ValidateContent(payload.Content)
	if err != nil {
		s.sendError(message.RequestID, "MESSAGE_TOO_LONG", "chat message is empty or too long")
		return
	}
	// 拿到房间对象。理论上到这一步一定拿得到，因为 join 时刚建过；防御性判断一下。
	roomState, ok := s.server.hub.GetRoom(s.roomID)
	if !ok {
		s.sendError(message.RequestID, "ROOM_NOT_FOUND", "room not found")
		return
	}
	now := unixMillis(time.Now())
	// 广播给房间里所有人（包括自己，这样发送方也能看到自己的消息回显）。
	roomState.Broadcast(signaling.Message{
		Type:      signaling.TypeChatMessage,
		RoomID:    s.roomID,
		PeerID:    s.peer.PeerID,
		UserID:    s.peer.UserID,
		Username:  s.peer.Username,
		Timestamp: now,
		Payload: signaling.Payload(map[string]interface{}{
			"messageId": "msg-" + uuid.NewString(), // 每条消息一个唯一 ID
			"content":   content,
		}),
	})
}

// close 是"离开房间 + 关闭连接"的清理函数。
// 会被以下路径调用：
//   1. run 的 defer（连接断开或 goroutine 退出时）
//   2. 收到 room.leave 消息时
// 因此可能被调用多次；我们把 peer 置 nil 使第二次调用变成"安全的空操作"。
func (s *wsSession) close(ctx context.Context) {
	if s.peer != nil {
		if roomState, ok := s.server.hub.GetRoom(s.roomID); ok {
			// 从房间成员表里删除自己，同时拿到"删除后还剩多少人"。
			peer, removed, remaining := roomState.RemovePeer(s.peer.PeerID)
			if removed {
				// 广播给房间里剩下的所有人："这个 peer 走了"。
				roomState.Broadcast(signaling.Message{
					Type:      signaling.TypePeerLeft,
					RoomID:    s.roomID,
					PeerID:    peer.PeerID,
					UserID:    peer.UserID,
					Username:  peer.Username,
					Timestamp: unixMillis(time.Now()),
					Payload:   signaling.Payload(peer.State()),
				})
			}
			// 房间空了 -> 从内存 Hub 移除，同时删除 Redis 里的元数据。
			// 这样一个"没人的房间"不会持续占用资源。
			if remaining == 0 {
				s.server.hub.DeleteRoom(s.roomID)
				_ = s.server.store.DeleteRoom(ctx, s.roomID)
			}
		}
		s.peer = nil
	}
	// 关闭底层 TCP/WS 连接。忽略错误：如果已经断开了 Close 也没什么可做的。
	_ = s.conn.Close()
}

// send 把任意 Go 值序列化成 JSON 发到客户端。
//
// 【重要：并发写保护】
// gorilla/websocket 的 Conn 明确要求"同一时刻只能有一个 goroutine 在写"。
// 广播/心跳/回执可能来自不同的 goroutine，所以这里必须用 s.mu 串行化写操作。
func (s *wsSession) send(value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conn.WriteJSON(value); err != nil {
		log.Printf("failed to write websocket message: %v", err)
	}
}

// sendError 通过 WebSocket 发一条 error 类型的信令消息给客户端。
func (s *wsSession) sendError(requestID string, code string, message string) {
	s.send(signaling.Message{
		Type:      signaling.TypeError,
		RequestID: requestID,
		RoomID:    s.roomID,
		PeerID:    s.peerID(),
		Timestamp: unixMillis(time.Now()),
		Payload:   signaling.Payload(ErrorResponse{Code: code, Message: message}),
	})
}

// peerID 是个便捷 getter：peer 还没建立时返回空字符串。
func (s *wsSession) peerID() string {
	if s.peer == nil {
		return ""
	}
	return s.peer.PeerID
}

// decodeRoomRequest 解析并校验 REST API 的请求体。
// 校验不通过时它会自己写 400 响应，返回 ok=false 让上层直接 return。
func decodeRoomRequest(w http.ResponseWriter, r *http.Request) (RoomRequest, bool) {
	var request RoomRequest
	// json.NewDecoder 比 json.Unmarshal 稍好一点：可以流式解，不用一次性把 body 读到内存。
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "bad request")
		return RoomRequest{}, false
	}
	request.Username = strings.TrimSpace(request.Username)
	// userId 必须为正、username 不能空、长度不能超过 32 个字符（不是字节）。
	if request.UserID <= 0 || request.Username == "" || len([]rune(request.Username)) > 32 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "incomplete user info")
		return RoomRequest{}, false
	}
	return request, true
}

// parseJoinPath 从形如 "/api/rooms/{id}/join" 的路径里抠出 roomID。
// 返回值：(roomID, ok)。
func parseJoinPath(path string) (string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/rooms/")
	// 如果 TrimPrefix 没有真的去掉前缀（说明前缀不匹配），trimmed 会等于原 path。
	// 或者末尾不是 "/join"，都视为路径不合法。
	if trimmed == path || !strings.HasSuffix(trimmed, "/join") {
		return "", false
	}
	roomID := strings.TrimSuffix(trimmed, "/join")
	return roomID, roomID != ""
}

// writeStateError 把 state 层的错误翻译成合适的 HTTP 响应。
//
// 【switch 里的空表达式 + case + 布尔】这是 Go 里一种优雅写法，
// 相当于 if / else if 链，可读性更好。
func writeStateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, state.ErrRoomNotFound):
		writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "room not found")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// writeJSON 是"写一个 JSON 响应"的小工具。
func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	// 【重要顺序】必须先 Set Header，再 WriteHeader，最后 Write body。
	// WriteHeader 一旦调用，Header 就不能再改了。
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// writeError 把 ErrorResponse 包装成一个 JSON 错误响应。
func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, ErrorResponse{Code: code, Message: message})
}

// withCORS 是一个"中间件"：接收一个 handler，返回一个"先加 CORS 响应头再调用它"的新 handler。
//
// 【什么是 CORS】跨域资源共享。浏览器出于安全会拒绝跨源请求，除非服务端明确说"允许你来"。
// 【生产环境建议】把 * 换成具体的前端域名，避免任何站点都能来调 API。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		// 【预检请求】浏览器对某些跨域请求会先发一个 OPTIONS 探路，
		// 拿到允许后再发真实请求。我们直接对 OPTIONS 返回 204 即可。
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// unixMillis 把 time.Time 转成毫秒时间戳。和 state 包里那个函数功能一致，
// 各自私有一份是为了避免包间循环依赖。
func unixMillis(value time.Time) int64 {
	return value.UnixNano() / int64(time.Millisecond)
}
