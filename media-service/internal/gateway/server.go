package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"streamforge/media-service/internal/chat"
	"streamforge/media-service/internal/config"
	"streamforge/media-service/internal/room"
	"streamforge/media-service/internal/router"
	"streamforge/media-service/internal/signaling"
)

type Server struct {
	cfg      config.Config
	store    router.Store
	hub      *room.Hub
	upgrader websocket.Upgrader
}

type RoomRequest struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
}

type RoomResponse struct {
	RoomID          string    `json:"roomId"`
	MediaInstanceID string    `json:"mediaInstanceId"`
	WSURL           string    `json:"wsUrl"`
	RTCConfig       RTCConfig `json:"rtcConfig"`
}

type RTCConfig struct {
	ICEServers []ICEServer `json:"iceServers"`
}

type ICEServer struct {
	URLs []string `json:"urls"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewServer(cfg config.Config, store router.Store, hub *room.Hub) *Server {
	return &Server{
		cfg:   cfg,
		store: store,
		hub:   hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/rooms", s.handleRooms)
	mux.HandleFunc("/api/rooms/", s.handleRoomAction)
	mux.HandleFunc("/ws/rooms/", s.handleWebSocket)
	return withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	rooms, peers := s.hub.Stats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "UP",
		"mediaInstanceId": s.cfg.MediaInstanceID,
		"rooms":           rooms,
		"peers":           peers,
		"redis":           "UP",
	})
}

func (s *Server) handleRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	request, ok := decodeRoomRequest(w, r)
	if !ok {
		return
	}
	roomID, err := s.generateRoomID(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "MEDIA_INSTANCE_UNAVAILABLE", "没有可用媒体实例")
		return
	}
	route, err := s.store.CreateRoomRoute(r.Context(), roomID, request.UserID)
	if err != nil {
		writeRouteError(w, err)
		return
	}
	if route.MediaInstanceID == s.cfg.MediaInstanceID {
		s.hub.EnsureRoom(roomID, request.UserID)
	}
	writeJSON(w, http.StatusOK, s.roomResponse(route.RoomID, route.MediaInstanceID))
}

func (s *Server) handleRoomAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	roomID, ok := parseJoinPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "接口不存在")
		return
	}
	if _, ok := decodeRoomRequest(w, r); !ok {
		return
	}
	route, err := s.store.GetRoomRoute(r.Context(), roomID)
	if err != nil {
		writeRouteError(w, err)
		return
	}
	if route.MediaInstanceID == s.cfg.MediaInstanceID {
		s.hub.EnsureRoom(roomID, 0)
	}
	writeJSON(w, http.StatusOK, s.roomResponse(route.RoomID, route.MediaInstanceID))
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimPrefix(r.URL.Path, "/ws/rooms/")
	if roomID == "" {
		writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "房间不存在")
		return
	}
	route, err := s.store.GetRoomRoute(r.Context(), roomID)
	if err != nil {
		writeRouteError(w, err)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	session := &wsSession{
		server: s,
		conn:   conn,
		roomID: roomID,
		route:  route,
	}
	session.run(r.Context())
}

func (s *Server) roomResponse(roomID string, mediaInstanceID string) RoomResponse {
	return RoomResponse{
		RoomID:          roomID,
		MediaInstanceID: mediaInstanceID,
		WSURL:           strings.TrimRight(s.cfg.PublicWSBaseURL, "/") + "/ws/rooms/" + roomID,
		RTCConfig: RTCConfig{ICEServers: []ICEServer{{
			URLs: s.cfg.ICEStunURLs,
		}}},
	}
}

func (s *Server) generateRoomID(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		roomID := strconv.Itoa(100000 + rand.Intn(900000))
		if _, err := s.store.GetRoomRoute(ctx, roomID); errors.Is(err, router.ErrRoomNotFound) {
			return roomID, nil
		}
	}
	return "", router.ErrMediaInstanceUnavailable
}

type wsSession struct {
	server *Server
	conn   *websocket.Conn
	roomID string
	route  router.RoomRoute
	peer   *room.Peer
	mu     sync.Mutex
}

func (s *wsSession) run(ctx context.Context) {
	defer s.close(ctx)
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		message, err := signaling.Decode(data)
		if err != nil {
			s.sendError("", "BAD_REQUEST", "请求格式错误")
			continue
		}
		s.handle(ctx, message)
	}
}

func (s *wsSession) handle(ctx context.Context, message signaling.Message) {
	switch message.Type {
	case signaling.TypeRoomJoin:
		s.handleJoin(message)
	case signaling.TypeRoomLeave:
		s.close(ctx)
	case signaling.TypeChatSend:
		s.handleChat(message)
	case signaling.TypePing:
		s.send(signaling.Message{
			Type:      signaling.TypePong,
			RequestID: message.RequestID,
			RoomID:    s.roomID,
			PeerID:    s.peerID(),
			UserID:    message.UserID,
			Username:  message.Username,
			Timestamp: time.Now().UnixMilli(),
			Payload:   signaling.Payload(map[string]string{}),
		})
	default:
		s.sendError(message.RequestID, "BAD_REQUEST", "不支持的消息类型")
	}
}

func (s *wsSession) handleJoin(message signaling.Message) {
	if s.route.MediaInstanceID != s.server.cfg.MediaInstanceID {
		s.sendError(message.RequestID, "ROOM_NOT_ON_INSTANCE", "当前连接的媒体实例不是该房间所属实例")
		return
	}
	if message.RoomID != "" && message.RoomID != s.roomID {
		s.sendError(message.RequestID, "BAD_REQUEST", "房间号不匹配")
		return
	}
	roomState := s.server.hub.EnsureRoom(s.roomID, message.UserID)
	peer := room.NewPeer("peer-"+uuid.NewString()[:8], message.UserID, message.Username)
	peer.SetSender(func(value interface{}) {
		s.send(value)
	})
	s.peer = peer
	roomState.AddPeer(peer)

	joined := signaling.Message{
		Type:      signaling.TypeRoomJoined,
		RequestID: message.RequestID,
		RoomID:    s.roomID,
		PeerID:    peer.PeerID,
		UserID:    peer.UserID,
		Username:  peer.Username,
		Timestamp: time.Now().UnixMilli(),
		Payload:   signaling.Payload(map[string]string{"mediaInstanceId": s.server.cfg.MediaInstanceID}),
	}
	s.send(joined)
	s.send(signaling.Message{
		Type:      signaling.TypeRoomSnapshot,
		RoomID:    s.roomID,
		PeerID:    peer.PeerID,
		UserID:    peer.UserID,
		Username:  peer.Username,
		Timestamp: time.Now().UnixMilli(),
		Payload:   signaling.Payload(roomState.Snapshot()),
	})
	roomState.BroadcastExcept(peer.PeerID, signaling.Message{
		Type:      signaling.TypePeerJoined,
		RoomID:    s.roomID,
		PeerID:    peer.PeerID,
		UserID:    peer.UserID,
		Username:  peer.Username,
		Timestamp: time.Now().UnixMilli(),
		Payload:   signaling.Payload(peer.State()),
	})
}

func (s *wsSession) handleChat(message signaling.Message) {
	if s.peer == nil {
		s.sendError(message.RequestID, "PEER_NOT_JOINED", "Peer 尚未加入房间")
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		s.sendError(message.RequestID, "BAD_REQUEST", "请求格式错误")
		return
	}
	content, err := chat.ValidateContent(payload.Content)
	if err != nil {
		s.sendError(message.RequestID, "MESSAGE_TOO_LONG", "聊天消息不能为空且不能超过1000字符")
		return
	}
	roomState, ok := s.server.hub.GetRoom(s.roomID)
	if !ok {
		s.sendError(message.RequestID, "ROOM_NOT_FOUND", "房间不存在")
		return
	}
	now := time.Now().UnixMilli()
	roomState.Broadcast(signaling.Message{
		Type:      signaling.TypeChatMessage,
		RoomID:    s.roomID,
		PeerID:    s.peer.PeerID,
		UserID:    s.peer.UserID,
		Username:  s.peer.Username,
		Timestamp: now,
		Payload: signaling.Payload(map[string]interface{}{
			"messageId": "msg-" + uuid.NewString(),
			"content":   content,
		}),
	})
}

func (s *wsSession) close(ctx context.Context) {
	if s.peer != nil {
		if roomState, ok := s.server.hub.GetRoom(s.roomID); ok {
			peer, removed, remaining := roomState.RemovePeer(s.peer.PeerID)
			if removed {
				roomState.Broadcast(signaling.Message{
					Type:      signaling.TypePeerLeft,
					RoomID:    s.roomID,
					PeerID:    peer.PeerID,
					UserID:    peer.UserID,
					Username:  peer.Username,
					Timestamp: time.Now().UnixMilli(),
					Payload:   signaling.Payload(peer.State()),
				})
			}
			if remaining == 0 {
				s.server.hub.DeleteRoom(s.roomID)
				_ = s.server.store.DeleteRoomRoute(ctx, s.roomID)
			}
		}
		s.peer = nil
	}
	_ = s.conn.Close()
}

func (s *wsSession) send(value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conn.WriteJSON(value); err != nil {
		slog.Debug("failed to write websocket message", "error", err)
	}
}

func (s *wsSession) sendError(requestID string, code string, message string) {
	s.send(signaling.Message{
		Type:      signaling.TypeError,
		RequestID: requestID,
		RoomID:    s.roomID,
		PeerID:    s.peerID(),
		Timestamp: time.Now().UnixMilli(),
		Payload:   signaling.Payload(ErrorResponse{Code: code, Message: message}),
	})
}

func (s *wsSession) peerID() string {
	if s.peer == nil {
		return ""
	}
	return s.peer.PeerID
}

func decodeRoomRequest(w http.ResponseWriter, r *http.Request) (RoomRequest, bool) {
	var request RoomRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
		return RoomRequest{}, false
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.UserID <= 0 || request.Username == "" || len([]rune(request.Username)) > 32 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "用户信息不完整")
		return RoomRequest{}, false
	}
	return request, true
}

func parseJoinPath(path string) (string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/rooms/")
	if trimmed == path || !strings.HasSuffix(trimmed, "/join") {
		return "", false
	}
	roomID := strings.TrimSuffix(trimmed, "/join")
	return roomID, roomID != ""
}

func writeRouteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, router.ErrRoomNotFound):
		writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "房间不存在")
	case errors.Is(err, router.ErrMediaInstanceUnavailable):
		writeError(w, http.StatusServiceUnavailable, "MEDIA_INSTANCE_UNAVAILABLE", "房间所属媒体实例不可用")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务端内部错误")
	}
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, ErrorResponse{Code: code, Message: message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
