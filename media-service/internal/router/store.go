package router

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrRoomNotFound             = errors.New("room not found")
	ErrMediaInstanceUnavailable = errors.New("media instance unavailable")
)

type InstanceInfo struct {
	ID              string
	Host            string
	HTTPPort        int
	WSPort          int
	RTCPortRange    string
	StartedAt       time.Time
	LastHeartbeatAt time.Time
	Status          string
	RoomCount       int
	PeerCount       int
}

type RoomRoute struct {
	RoomID          string
	MediaInstanceID string
	CreatedByUserID int64
	CreatedAt       time.Time
}

type Store interface {
	UpsertInstance(ctx context.Context, info InstanceInfo) error
	ListHealthyInstances(ctx context.Context, maxAge time.Duration) ([]InstanceInfo, error)
	CreateRoomRoute(ctx context.Context, roomID string, createdByUserID int64) (RoomRoute, error)
	GetRoomRoute(ctx context.Context, roomID string) (RoomRoute, error)
	DeleteRoomRoute(ctx context.Context, roomID string) error
}

type MemoryStore struct {
	mu        sync.Mutex
	instances map[string]InstanceInfo
	routes    map[string]RoomRoute
	next      int
	now       func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		instances: make(map[string]InstanceInfo),
		routes:    make(map[string]RoomRoute),
		now:       time.Now,
	}
}

func (s *MemoryStore) UpsertInstance(_ context.Context, info InstanceInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances[info.ID] = info
	return nil
}

func (s *MemoryStore) ListHealthyInstances(_ context.Context, maxAge time.Duration) ([]InstanceInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.healthyInstances(maxAge), nil
}

func (s *MemoryStore) CreateRoomRoute(_ context.Context, roomID string, createdByUserID int64) (RoomRoute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if route, ok := s.routes[roomID]; ok {
		return route, nil
	}
	instances := s.healthyInstances(30 * time.Second)
	if len(instances) == 0 {
		return RoomRoute{}, ErrMediaInstanceUnavailable
	}
	instance := instances[s.next%len(instances)]
	s.next++
	route := RoomRoute{
		RoomID:          roomID,
		MediaInstanceID: instance.ID,
		CreatedByUserID: createdByUserID,
		CreatedAt:       s.now(),
	}
	s.routes[roomID] = route
	return route, nil
}

func (s *MemoryStore) GetRoomRoute(_ context.Context, roomID string) (RoomRoute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	route, ok := s.routes[roomID]
	if !ok {
		return RoomRoute{}, ErrRoomNotFound
	}
	instance, ok := s.instances[route.MediaInstanceID]
	if !ok || !isHealthy(instance, s.now(), 30*time.Second) {
		return RoomRoute{}, ErrMediaInstanceUnavailable
	}
	return route, nil
}

func (s *MemoryStore) DeleteRoomRoute(_ context.Context, roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.routes, roomID)
	return nil
}

func (s *MemoryStore) healthyInstances(maxAge time.Duration) []InstanceInfo {
	instances := make([]InstanceInfo, 0, len(s.instances))
	for _, instance := range s.instances {
		if isHealthy(instance, s.now(), maxAge) {
			instances = append(instances, instance)
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ID < instances[j].ID
	})
	return instances
}

func isHealthy(instance InstanceInfo, now time.Time, maxAge time.Duration) bool {
	if instance.ID == "" || instance.Status != "healthy" {
		return false
	}
	return !instance.LastHeartbeatAt.IsZero() && now.Sub(instance.LastHeartbeatAt) <= maxAge
}
