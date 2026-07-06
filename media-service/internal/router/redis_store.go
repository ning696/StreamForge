package router

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	instancesKey    = "streamforge:media:instances"
	routeCounterKey = "streamforge:room:route:counter"
)

type RedisStore struct {
	client *redis.Client
	now    func() time.Time
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, now: time.Now}
}

func NewRedisClient(addr string, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) UpsertInstance(ctx context.Context, info InstanceInfo) error {
	key := instanceKey(info.ID)
	nowMillis := info.LastHeartbeatAt.UnixMilli()
	values := map[string]interface{}{
		"id":              info.ID,
		"host":            info.Host,
		"httpPort":        strconv.Itoa(info.HTTPPort),
		"wsPort":          strconv.Itoa(info.WSPort),
		"rtcPortRange":    info.RTCPortRange,
		"status":          info.Status,
		"startedAt":       strconv.FormatInt(info.StartedAt.UnixMilli(), 10),
		"lastHeartbeatAt": strconv.FormatInt(nowMillis, 10),
		"roomCount":       strconv.Itoa(info.RoomCount),
		"peerCount":       strconv.Itoa(info.PeerCount),
	}
	if err := s.client.SAdd(ctx, instancesKey, info.ID).Err(); err != nil {
		return err
	}
	if err := s.client.HSet(ctx, key, values).Err(); err != nil {
		return err
	}
	return s.client.Expire(ctx, key, 30*time.Second).Err()
}

func (s *RedisStore) ListHealthyInstances(ctx context.Context, maxAge time.Duration) ([]InstanceInfo, error) {
	ids, err := s.client.SMembers(ctx, instancesKey).Result()
	if err != nil {
		return nil, err
	}
	instances := make([]InstanceInfo, 0, len(ids))
	now := s.now()
	for _, id := range ids {
		info, err := s.loadInstance(ctx, id)
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, err
		}
		if isHealthy(info, now, maxAge) {
			instances = append(instances, info)
		}
	}
	return instances, nil
}

func (s *RedisStore) CreateRoomRoute(ctx context.Context, roomID string, createdByUserID int64) (RoomRoute, error) {
	if route, err := s.GetRoomRoute(ctx, roomID); err == nil {
		return route, nil
	} else if err != ErrRoomNotFound {
		return RoomRoute{}, err
	}
	instances, err := s.ListHealthyInstances(ctx, 30*time.Second)
	if err != nil {
		return RoomRoute{}, err
	}
	if len(instances) == 0 {
		return RoomRoute{}, ErrMediaInstanceUnavailable
	}
	counter, err := s.client.Incr(ctx, routeCounterKey).Result()
	if err != nil {
		return RoomRoute{}, err
	}
	instance := instances[int(counter-1)%len(instances)]
	route := RoomRoute{
		RoomID:          roomID,
		MediaInstanceID: instance.ID,
		CreatedByUserID: createdByUserID,
		CreatedAt:       s.now(),
	}
	if err := s.client.Set(ctx, routeKey(roomID), route.MediaInstanceID, 24*time.Hour).Err(); err != nil {
		return RoomRoute{}, err
	}
	meta := map[string]interface{}{
		"roomId":          roomID,
		"mediaInstanceId": route.MediaInstanceID,
		"createdByUserId": strconv.FormatInt(createdByUserID, 10),
		"createdAt":       strconv.FormatInt(route.CreatedAt.UnixMilli(), 10),
		"peerCount":       "0",
		"status":          "active",
	}
	_ = s.client.HSet(ctx, roomMetaKey(roomID), meta).Err()
	_ = s.client.Expire(ctx, roomMetaKey(roomID), 24*time.Hour).Err()
	return route, nil
}

func (s *RedisStore) GetRoomRoute(ctx context.Context, roomID string) (RoomRoute, error) {
	instanceID, err := s.client.Get(ctx, routeKey(roomID)).Result()
	if err == redis.Nil {
		return RoomRoute{}, ErrRoomNotFound
	}
	if err != nil {
		return RoomRoute{}, err
	}
	info, err := s.loadInstance(ctx, instanceID)
	if err == redis.Nil {
		return RoomRoute{}, ErrMediaInstanceUnavailable
	}
	if err != nil {
		return RoomRoute{}, err
	}
	if !isHealthy(info, s.now(), 30*time.Second) {
		return RoomRoute{}, ErrMediaInstanceUnavailable
	}
	return RoomRoute{RoomID: roomID, MediaInstanceID: instanceID}, nil
}

func (s *RedisStore) DeleteRoomRoute(ctx context.Context, roomID string) error {
	return s.client.Del(ctx, routeKey(roomID), roomMetaKey(roomID), roomPeersKey(roomID)).Err()
}

func (s *RedisStore) loadInstance(ctx context.Context, id string) (InstanceInfo, error) {
	values, err := s.client.HGetAll(ctx, instanceKey(id)).Result()
	if err != nil {
		return InstanceInfo{}, err
	}
	if len(values) == 0 {
		return InstanceInfo{}, redis.Nil
	}
	lastHeartbeatAt := parseMillis(values["lastHeartbeatAt"])
	startedAt := parseMillis(values["startedAt"])
	httpPort, _ := strconv.Atoi(values["httpPort"])
	wsPort, _ := strconv.Atoi(values["wsPort"])
	roomCount, _ := strconv.Atoi(values["roomCount"])
	peerCount, _ := strconv.Atoi(values["peerCount"])
	return InstanceInfo{
		ID:              values["id"],
		Host:            values["host"],
		HTTPPort:        httpPort,
		WSPort:          wsPort,
		RTCPortRange:    values["rtcPortRange"],
		StartedAt:       startedAt,
		LastHeartbeatAt: lastHeartbeatAt,
		Status:          values["status"],
		RoomCount:       roomCount,
		PeerCount:       peerCount,
	}, nil
}

func parseMillis(value string) time.Time {
	millis, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(millis)
}

func instanceKey(id string) string {
	return "streamforge:media:instance:" + id
}

func routeKey(roomID string) string {
	return "streamforge:room:" + roomID + ":instance"
}

func roomMetaKey(roomID string) string {
	return "streamforge:room:" + roomID + ":meta"
}

func roomPeersKey(roomID string) string {
	return "streamforge:room:" + roomID + ":peers"
}
