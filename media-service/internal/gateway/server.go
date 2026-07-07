package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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
	livekittoken "streamforge/media-service/internal/livekit"
	"streamforge/media-service/internal/room"
	"streamforge/media-service/internal/signaling"
	"streamforge/media-service/internal/state"
)

type Server struct {
	cfg      config.Config
	store    state.Store
	issuer   *livekittoken.Issuer
	hub      *room.Hub
	upgrader websocket.Upgrader
}

type RoomRequest struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
}

type RoomResponse struct {
	RoomID          string `json:"roomId"`
	LiveKitRoomName string `json:"livekitRoomName"`
	LiveKitURL      string `json:"livekitUrl"`
	LiveKitToken    string `json:"livekitToken"`
	LiveKitIdentity string `json:"livekitIdentity"`
	AppWSURL        string `json:"appWsUrl"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewServer(cfg config.Config, store state.Store, issuer *livekittoken.Issuer, hub *room.Hub) *Server {
	return &Server{
		cfg:    cfg,
		store:  store,
		issuer: issuer,
		hub:    hub,
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
		"status":            "UP",
		"livekitConfigured": s.issuer != nil && s.issuer.Configured(),
		"rooms":             rooms,
		"appPeers":          peers,
		"redis":             "UP",
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
		writeStateError(w, err)
		return
	}
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
	s.hub.EnsureRoom(roomID, request.UserID)
	response, err := s.roomResponse(meta, request)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "LIVEKIT_UNAVAILABLE", "livekit is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRoomAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	roomID, ok := parseJoinPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	request, ok := decodeRoomRequest(w, r)
	if !ok {
		return
	}
	meta, err := s.store.GetRoom(r.Context(), roomID)
	if err != nil {
		writeStateError(w, err)
		return
	}
	s.hub.EnsureRoom(roomID, 0)
	response, err := s.roomResponse(meta, request)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "LIVEKIT_UNAVAILABLE", "livekit is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimPrefix(r.URL.Path, "/ws/rooms/")
	if roomID == "" {
		writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "room not found")
		return
	}
	meta, err := s.store.GetRoom(r.Context(), roomID)
	if err != nil {
		writeStateError(w, err)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	session := &wsSession{server: s, conn: conn, roomID: roomID, meta: meta}
	session.run(r.Context())
}

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
		AppWSURL:        strings.TrimRight(s.cfg.PublicWSBaseURL, "/") + "/ws/rooms/" + meta.RoomID,
	}, nil
}

func (s *Server) generateRoomID(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		roomID := strconv.Itoa(100000 + rand.Intn(900000))
		if _, err := s.store.GetRoom(ctx, roomID); errors.Is(err, state.ErrRoomNotFound) {
			return roomID, nil
		}
	}
	return "", errors.New("generate room id failed")
}

type wsSession struct {
	server *Server
	conn   *websocket.Conn
	roomID string
	meta   state.RoomMeta
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
			s.sendError("", "BAD_REQUEST", "bad request")
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
			Timestamp: unixMillis(time.Now()),
			Payload:   signaling.Payload(map[string]string{}),
		})
	default:
		s.sendError(message.RequestID, "BAD_REQUEST", "unsupported message type")
	}
}

func (s *wsSession) handleJoin(message signaling.Message) {
	if message.RoomID != "" && message.RoomID != s.roomID {
		s.sendError(message.RequestID, "BAD_REQUEST", "room id mismatch")
		return
	}
	var payload struct {
		LiveKitIdentity string `json:"livekitIdentity"`
	}
	_ = json.Unmarshal(message.Payload, &payload)
	roomState := s.server.hub.EnsureRoom(s.roomID, message.UserID)
	peer := room.NewPeer("app-peer-"+uuid.NewString()[:8], message.UserID, message.Username, payload.LiveKitIdentity)
	peer.SetSender(func(value interface{}) { s.send(value) })
	s.peer = peer
	roomState.AddPeer(peer)

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
	s.send(signaling.Message{
		Type:      signaling.TypeRoomSnapshot,
		RoomID:    s.roomID,
		PeerID:    peer.PeerID,
		UserID:    peer.UserID,
		Username:  peer.Username,
		Timestamp: unixMillis(time.Now()),
		Payload:   signaling.Payload(roomState.Snapshot()),
	})
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

func (s *wsSession) handleChat(message signaling.Message) {
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
	content, err := chat.ValidateContent(payload.Content)
	if err != nil {
		s.sendError(message.RequestID, "MESSAGE_TOO_LONG", "chat message is empty or too long")
		return
	}
	roomState, ok := s.server.hub.GetRoom(s.roomID)
	if !ok {
		s.sendError(message.RequestID, "ROOM_NOT_FOUND", "room not found")
		return
	}
	now := unixMillis(time.Now())
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
					Timestamp: unixMillis(time.Now()),
					Payload:   signaling.Payload(peer.State()),
				})
			}
			if remaining == 0 {
				s.server.hub.DeleteRoom(s.roomID)
				_ = s.server.store.DeleteRoom(ctx, s.roomID)
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
		log.Printf("failed to write websocket message: %v", err)
	}
}

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

func (s *wsSession) peerID() string {
	if s.peer == nil {
		return ""
	}
	return s.peer.PeerID
}

func decodeRoomRequest(w http.ResponseWriter, r *http.Request) (RoomRequest, bool) {
	var request RoomRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "bad request")
		return RoomRequest{}, false
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.UserID <= 0 || request.Username == "" || len([]rune(request.Username)) > 32 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "incomplete user info")
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

func writeStateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, state.ErrRoomNotFound):
		writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "room not found")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
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

func unixMillis(value time.Time) int64 {
	return value.UnixNano() / int64(time.Millisecond)
}
