// Package state 负责"房间元信息"的持久化。
//
// 【为什么要有一个 Store 接口】
// 房间元信息需要跨请求、甚至跨服务实例共享（比如 A 实例创建的房间，B 实例接收到 WS 连接
// 也要能查到它），所以要落到 Redis 这样的外部存储里。
// 但在写单元测试的时候，我们不希望依赖真的 Redis。因此定义了一个 Store 接口，
// 生产用 RedisStore，测试用 MemoryStore（本文件下面就是），业务代码只依赖接口。
// 这就是"依赖倒置"——上层只认接口，底层实现可以随便换。
package state

import (
	"context" // Go 里贯穿整个调用链的"上下文"，用来传超时/取消信号
	"errors"  // 生成 error 值
	"sync"    // 提供 Mutex 等并发原语
	"time"    // 时间相关
)

// ErrRoomNotFound 是"房间不存在"的语义错误。
//
// 【为什么单独定义一个 error 值】
// 上层调用者通过 errors.Is(err, state.ErrRoomNotFound) 就能判断"房间不存在"这个具体情况，
// 从而返回 404 而不是 500。用字符串比较是脆弱且不推荐的做法。
var ErrRoomNotFound = errors.New("room not found")

// RoomMeta 房间元信息——只包含"房间本身"的属性，不含房间内的成员列表。
// 成员实时状态在 room.Hub 里保存（内存里，因为进程重启后成员必然要重连）。
type RoomMeta struct {
	RoomID          string    // 6 位数字的房间 ID，例如 "123456"
	LiveKitRoomName string    // 对应到 LiveKit 那边的房间名，通常是 "streamforge-{RoomID}"
	CreatedByUserID int64     // 创建者的用户 ID
	CreatedAt       time.Time // 创建时间
	Status          string    // 房间状态，例如 "active"
}

// Store 是房间元信息存储的抽象接口。
//
// 【接口设计原则】只暴露最小必需的方法。这里只有增删查三个，没有 Update——
// 因为业务里房间元信息一旦创建基本不变，如果确实要改，可以先 Delete 再 Create。
type Store interface {
	// CreateRoom 创建（或幂等更新）一个房间。ctx 用于超时控制。
	CreateRoom(ctx context.Context, meta RoomMeta) error
	// GetRoom 根据房间 ID 取回元信息。找不到时返回 ErrRoomNotFound。
	GetRoom(ctx context.Context, roomID string) (RoomMeta, error)
	// DeleteRoom 删除房间。删除不存在的房间应视为成功（幂等）。
	DeleteRoom(ctx context.Context, roomID string) error
}

// MemoryStore 是 Store 接口的"内存实现"，主要用于单元测试和本地快速跑通。
//
// 【为什么不直接用 map】
// map 在 Go 里"并发读写不安全"——同时读写会 panic。
// 由于 HTTP handler 是每个请求一个 goroutine，多个 goroutine 会同时访问这个 map，
// 所以要用 sync.Mutex 加锁保护。
type MemoryStore struct {
	mu    sync.Mutex          // 互斥锁，保护下面的 map
	rooms map[string]RoomMeta // roomID -> 元信息
}

// NewMemoryStore 是 MemoryStore 的构造函数。
//
// 【Go 约定】NewXxx 函数是"构造函数"的惯用命名。返回指针是因为：
//   1. MemoryStore 包含 sync.Mutex，拷贝会让锁失效（很危险）
//   2. 我们希望所有 handler 共享同一个 store 实例
func NewMemoryStore() *MemoryStore {
	// 【小细节】必须显式 make(map[...]...)，Go 里未初始化的 map 是 nil，向 nil map 写入会 panic。
	return &MemoryStore{rooms: make(map[string]RoomMeta)}
}

// CreateRoom 创建或"合并式更新"一个房间。
//
// 【接收器 (s *MemoryStore)】方法定义在指针接收器上，
// 才能读写 s.rooms（值接收器会拷贝一份 struct，改的是副本）。
// 参数里的 _ context.Context 表示"我不用这个参数但要保持接口签名一致"。
func (s *MemoryStore) CreateRoom(_ context.Context, meta RoomMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock() // defer 保证函数退出时一定解锁，避免忘记解锁导致死锁

	// 如果房间已经存在，走"合并"逻辑而不是直接覆盖。
	// 这样"WS 会话补充 LiveKit 房间名"这类局部更新不会把原来的 CreatedByUserID 抹掉。
	if existing, ok := s.rooms[meta.RoomID]; ok {
		// 特殊情况：新数据只有 RoomID 没有 LiveKitRoomName，视为"什么都不用改"，直接返回。
		if meta.LiveKitRoomName == "" {
			return nil
		}
		s.rooms[meta.RoomID] = mergeRoomMeta(existing, meta)
		return nil
	}

	// 全新的房间，直接写入。
	s.rooms[meta.RoomID] = meta
	return nil
}

// GetRoom 查一个房间。找不到返回 ErrRoomNotFound。
func (s *MemoryStore) GetRoom(_ context.Context, roomID string) (RoomMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 【Go 惯用法】"comma ok idiom"：map 取值时用两个返回值区分"取到了 vs 键不存在"。
	meta, ok := s.rooms[roomID]
	if !ok {
		return RoomMeta{}, ErrRoomNotFound
	}
	return meta, nil
}

// DeleteRoom 删除房间。
//
// 【幂等】即便 roomID 不存在，delete 也不会报错（Go 语言层面就允许），我们直接返回 nil。
func (s *MemoryStore) DeleteRoom(_ context.Context, roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, roomID)
	return nil
}

// mergeRoomMeta 把 next（新的、可能只填了部分字段）合并到 existing（老的）上。
//
// 【为什么这样写】允许调用方"只更新我关心的字段"——传入的 next 里字段为零值的部分
// 会被跳过，保留 existing 里的值。这是一种在 Go 里表达"partial update"的常见做法，
// 缺点是无法表达"我想把这个字段清零"，但对我们业务足够。
//
// 【小写函数】mergeRoomMeta 首字母小写，意味着这是"包内私有"函数，
// 外部包引用不到。这类工具函数没必要暴露。
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
	// time.Time 的零值不是 0，要用 IsZero() 判断。
	if !next.CreatedAt.IsZero() {
		existing.CreatedAt = next.CreatedAt
	}
	if next.Status != "" {
		existing.Status = next.Status
	}
	return existing
}
