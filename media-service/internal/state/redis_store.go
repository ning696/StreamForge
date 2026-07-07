package state

import (
	"context"
	"strconv"
	"time"

	redis "github.com/go-redis/redis"
)

const roomMetaTTL = 24 * time.Hour

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
	return s.client.Ping().Err()
}

func (s *RedisStore) CreateRoom(ctx context.Context, meta RoomMeta) error {
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = s.now()
	}
	if meta.Status == "" {
		meta.Status = "active"
	}
	values := map[string]interface{}{
		"roomId":          meta.RoomID,
		"livekitRoomName": meta.LiveKitRoomName,
		"createdByUserId": strconv.FormatInt(meta.CreatedByUserID, 10),
		"createdAt":       strconv.FormatInt(unixMillis(meta.CreatedAt), 10),
		"status":          meta.Status,
	}
	key := roomMetaKey(meta.RoomID)
	if err := s.client.HMSet(key, values).Err(); err != nil {
		return err
	}
	return s.client.Expire(key, roomMetaTTL).Err()
}

func (s *RedisStore) GetRoom(ctx context.Context, roomID string) (RoomMeta, error) {
	values, err := s.client.HGetAll(roomMetaKey(roomID)).Result()
	if err != nil {
		return RoomMeta{}, err
	}
	if len(values) == 0 {
		return RoomMeta{}, ErrRoomNotFound
	}
	createdBy, _ := strconv.ParseInt(values["createdByUserId"], 10, 64)
	createdAtMillis, _ := strconv.ParseInt(values["createdAt"], 10, 64)
	return RoomMeta{
		RoomID:          values["roomId"],
		LiveKitRoomName: values["livekitRoomName"],
		CreatedByUserID: createdBy,
		CreatedAt:       time.Unix(0, createdAtMillis*int64(time.Millisecond)),
		Status:          values["status"],
	}, nil
}

func (s *RedisStore) DeleteRoom(ctx context.Context, roomID string) error {
	return s.client.Del(roomMetaKey(roomID), roomPeersKey(roomID)).Err()
}

func roomMetaKey(roomID string) string {
	return "streamforge:room:" + roomID + ":meta"
}

func roomPeersKey(roomID string) string {
	return "streamforge:room:" + roomID + ":peers"
}

func unixMillis(value time.Time) int64 {
	return value.UnixNano() / int64(time.Millisecond)
}
