package state

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrRoomNotFound = errors.New("room not found")

type RoomMeta struct {
	RoomID          string
	LiveKitRoomName string
	CreatedByUserID int64
	CreatedAt       time.Time
	Status          string
}

type Store interface {
	CreateRoom(ctx context.Context, meta RoomMeta) error
	GetRoom(ctx context.Context, roomID string) (RoomMeta, error)
	DeleteRoom(ctx context.Context, roomID string) error
}

type MemoryStore struct {
	mu    sync.Mutex
	rooms map[string]RoomMeta
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{rooms: make(map[string]RoomMeta)}
}

func (s *MemoryStore) CreateRoom(_ context.Context, meta RoomMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.rooms[meta.RoomID]; ok {
		if meta.LiveKitRoomName == "" {
			return nil
		}
		s.rooms[meta.RoomID] = mergeRoomMeta(existing, meta)
		return nil
	}
	s.rooms[meta.RoomID] = meta
	return nil
}

func (s *MemoryStore) GetRoom(_ context.Context, roomID string) (RoomMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.rooms[roomID]
	if !ok {
		return RoomMeta{}, ErrRoomNotFound
	}
	return meta, nil
}

func (s *MemoryStore) DeleteRoom(_ context.Context, roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, roomID)
	return nil
}

func mergeRoomMeta(existing RoomMeta, next RoomMeta) RoomMeta {
	if next.RoomID != "" {
		existing.RoomID = next.RoomID
	}
	if next.LiveKitRoomName != "" {
		existing.LiveKitRoomName = next.LiveKitRoomName
	}
	if next.CreatedByUserID != 0 {
		existing.CreatedByUserID = next.CreatedByUserID
	}
	if !next.CreatedAt.IsZero() {
		existing.CreatedAt = next.CreatedAt
	}
	if next.Status != "" {
		existing.Status = next.Status
	}
	return existing
}
