package state

import (
	"context"
	"strconv"
	"time"

	// 这是 Redis 官方的 Go 客户端库。
	// import 时的别名 redis 是为了让下方代码可以写 redis.NewClient(...) 这种简短形式。
	redis "github.com/go-redis/redis"
)

// roomMetaTTL 是房间元数据在 Redis 里的过期时间。
//
// 【为什么要设过期】
// Redis 是内存数据库，容量宝贵。房间开着的时候我们会持续续期或直接删除；
// 但万一进程崩溃、没有正常清理，这个 TTL 就是"最后一道防线"——过 24 小时自动清掉，
// 防止 Redis 里堆满"僵尸房间"。
const roomMetaTTL = 24 * time.Hour

// RedisStore 是 Store 接口的"Redis 实现"，用于生产环境。
//
// 【为什么要注入 now 函数】
// 直接用 time.Now() 会让"依赖当前时间"的行为难以测试。
// 把 now 抽成一个字段，测试时可以替换成"永远返回固定时间"的假函数，
// 让测试结果可复现。这是 Go 里做"时间相关测试"的常见技巧。
type RedisStore struct {
	client *redis.Client   // Redis 客户端连接
	now    func() time.Time // "获取当前时间"的函数，方便测试打桩
}

// NewRedisStore 构造一个 RedisStore。
// 默认用 time.Now 作为时间源；测试里可以直接给结构体字段赋值来替换。
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, now: time.Now}
}

// NewRedisClient 是一个便捷函数：根据配置创建一个 *redis.Client。
//
// 【为什么放在这里】RedisStore 依赖 *redis.Client。上层（main.go）需要先创建 client
// 再交给 RedisStore。把创建逻辑放在 state 包里，让 main.go 不用直接 import redis 库。
func NewRedisClient(addr string, password string, db int) *redis.Client {
	// &redis.Options{...} 用结构体字面量创建配置并取地址，Go 里常见的"构造 + 配置"模式。
	return redis.NewClient(&redis.Options{
		Addr:     addr,     // Redis 地址，如 "127.0.0.1:6379"
		Password: password, // 密码（可能为空）
		DB:       db,       // DB 编号 0-15
	})
}

// Ping 用来做健康检查——探测和 Redis 的连接是否活着。
//
// 【小知识】这个包用的 go-redis 是较老的版本（v6），
// 它的 API 不接收 context，所以这里 ctx 参数目前没有传下去。
// 生产项目通常会升级到 v8/v9 版本，可以把超时信号真正传给底层调用。
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping().Err()
}

// CreateRoom 把房间元信息写入 Redis。
//
// 【Redis 数据结构选择】用 Hash（哈希表）保存一个房间的字段，key 是 "streamforge:room:{roomID}:meta"，
// 里面每个字段（roomId、livekitRoomName...）对应哈希里的一个"小键值"。
// 相比"把整个 JSON 塞进 String"，Hash 支持"只读取/更新某个字段"，未来更灵活。
func (s *RedisStore) CreateRoom(ctx context.Context, meta RoomMeta) error {
	// 如果调用方没设置创建时间，我们给它填上"现在"。
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = s.now()
	}
	// 状态默认 active。
	if meta.Status == "" {
		meta.Status = "active"
	}

	// Redis Hash 的值必须是字符串，所以数字要用 strconv 转成字符串再存。
	// map[string]interface{} 允许 value 是任意类型；这里我们统一放成 string。
	values := map[string]interface{}{
		"roomId":          meta.RoomID,
		"livekitRoomName": meta.LiveKitRoomName,
		"createdByUserId": strconv.FormatInt(meta.CreatedByUserID, 10),
		// 把时间转成毫秒时间戳存储（跨语言、跨时区最简单最通用）。
		"createdAt": strconv.FormatInt(unixMillis(meta.CreatedAt), 10),
		"status":    meta.Status,
	}

	key := roomMetaKey(meta.RoomID)

	// HMSET：一次性写入多个 hash 字段。
	// .Err() 从命令结果里取出 error。
	if err := s.client.HMSet(key, values).Err(); err != nil {
		return err
	}

	// 给这个 key 设置过期时间。EXPIRE 是"从现在起 24 小时后自动删除"。
	return s.client.Expire(key, roomMetaTTL).Err()
}

// GetRoom 从 Redis 读回房间元信息。
func (s *RedisStore) GetRoom(ctx context.Context, roomID string) (RoomMeta, error) {
	// HGETALL：读取该 Hash 的所有字段。返回 map[string]string。
	values, err := s.client.HGetAll(roomMetaKey(roomID)).Result()
	if err != nil {
		return RoomMeta{}, err
	}

	// 【关键行为差异】Redis 里 key 不存在时 HGETALL 返回"空 map"而不是错误。
	// 我们要把这种情况显式翻译成 ErrRoomNotFound，让上层能识别成 404。
	if len(values) == 0 {
		return RoomMeta{}, ErrRoomNotFound
	}

	// 用 "comma ok" 模式取值时忽略 ok 的位置，这里用 _ 忽略解析错误——
	// 如果字段格式错乱（几乎不会发生），退化为 0，不阻塞主流程。
	createdBy, _ := strconv.ParseInt(values["createdByUserId"], 10, 64)
	createdAtMillis, _ := strconv.ParseInt(values["createdAt"], 10, 64)

	return RoomMeta{
		RoomID:          values["roomId"],
		LiveKitRoomName: values["livekitRoomName"],
		CreatedByUserID: createdBy,
		// 把毫秒时间戳还原成 time.Time。
		// time.Unix(sec, nsec) 需要秒 + 纳秒；我们只有毫秒，就把毫秒 * 1e6 转成纳秒。
		CreatedAt: time.Unix(0, createdAtMillis*int64(time.Millisecond)),
		Status:    values["status"],
	}, nil
}

// DeleteRoom 同时删掉房间的元数据 key 和成员列表 key。
//
// 【为什么一次删两个】即使这个版本还没用到 peers key，
// 未来一旦启用了成员持久化，这里也应该一起清理，避免残留。
func (s *RedisStore) DeleteRoom(ctx context.Context, roomID string) error {
	return s.client.Del(roomMetaKey(roomID), roomPeersKey(roomID)).Err()
}

// roomMetaKey 拼装 "房间元数据" 在 Redis 里的完整 key。
//
// 【命名空间约定】前缀 "streamforge:" 用来避免和其他应用共用一个 Redis 时的 key 冲突；
// ":room:{id}:meta" 是分层命名，便于运维用 SCAN 通配符查找："streamforge:room:*:meta"。
func roomMetaKey(roomID string) string {
	return "streamforge:room:" + roomID + ":meta"
}

// roomPeersKey 拼装"房间成员列表"在 Redis 里的 key。当前未使用但预留出来。
func roomPeersKey(roomID string) string {
	return "streamforge:room:" + roomID + ":peers"
}

// unixMillis 把 time.Time 转换成"Unix 毫秒时间戳"。
//
// 【为什么不用 t.UnixMilli()】那是 Go 1.17 才有的方法；本项目可能考虑兼容更老的版本，
// 所以手写成 UnixNano() / 1e6 的形式。功能完全等价。
func unixMillis(value time.Time) int64 {
	return value.UnixNano() / int64(time.Millisecond)
}
