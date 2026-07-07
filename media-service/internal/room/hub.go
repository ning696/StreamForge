package room

import (
	"sync"
	"time"
)

type PeerState struct {
	PeerID          string `json:"peerId"`
	UserID          int64  `json:"userId"`
	Username        string `json:"username"`
	LiveKitIdentity string `json:"livekitIdentity,omitempty"`
	AudioEnabled    bool   `json:"audioEnabled"`
	VideoEnabled    bool   `json:"videoEnabled"`
	ScreenSharing   bool   `json:"screenSharing"`
}

type Snapshot struct {
	Peers []PeerState `json:"peers"`
}

type ChatMessage struct {
	MessageID string `json:"messageId"`
	RoomID    string `json:"roomId"`
	PeerID    string `json:"peerId"`
	UserID    int64  `json:"userId"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	SentAt    int64  `json:"sentAt"`
}

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
	send            func(interface{})
	outbox          []interface{}
}

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

func (p *Peer) SetSender(send func(interface{})) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.send = send
}

func (p *Peer) Send(message interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.send != nil {
		p.send(message)
		return
	}
	p.outbox = append(p.outbox, message)
}

func (p *Peer) Outbox() []interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]interface{}(nil), p.outbox...)
}

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

type Room struct {
	mu              sync.Mutex
	RoomID          string
	CreatedByUserID int64
	CreatedAt       time.Time
	peers           map[string]*Peer
}

func NewRoom(roomID string, createdByUserID int64) *Room {
	return &Room{
		RoomID:          roomID,
		CreatedByUserID: createdByUserID,
		CreatedAt:       time.Now(),
		peers:           make(map[string]*Peer),
	}
}

func (r *Room) AddPeer(peer *Peer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[peer.PeerID] = peer
}

func (r *Room) RemovePeer(peerID string) (*Peer, bool, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	peer, ok := r.peers[peerID]
	delete(r.peers, peerID)
	return peer, ok, len(r.peers)
}

func (r *Room) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	peers := make([]PeerState, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer.State())
	}
	return Snapshot{Peers: peers}
}

func (r *Room) RecordChat(message ChatMessage) {
	r.Broadcast(message)
}

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

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

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

func (h *Hub) GetRoom(roomID string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	roomState, ok := h.rooms[roomID]
	return roomState, ok
}

func (h *Hub) DeleteRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, roomID)
}

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
