// Package livekit 封装了和 LiveKit 服务器打交道所需的 Token 签发逻辑。
//
// 【背景】
// LiveKit 是一款开源的 WebRTC SFU 服务。它不允许匿名接入，客户端要连上去必须
// 先拿到一个 JWT Token，Token 里说明了"我是谁""可以进哪个房间""什么权限"。
// 这个 Token 必须由"知道 API Secret 的服务端"来签发，绝对不能把 Secret 泄漏给前端。
//
// 【本文件做什么】
// 1. 定义 Token 请求 / 结果的数据结构。
// 2. 提供 Issuer（签发者）来生成符合 LiveKit 规范的 JWT。
// 3. 实现 HS256 签名的 JWT（不依赖第三方 JWT 库，代码更透明）。
package livekit

import (
	"crypto/hmac"     // HMAC 消息认证码，用来做 JWT 签名
	"crypto/sha256"   // SHA-256 哈希算法，与 HMAC 配合形成 HS256
	"encoding/base64" // JWT 用 base64url 编码 header/payload/signature
	"encoding/json"   // 序列化 header 和 claims
	"errors"
	"fmt"
	"strings"
	"time"

	// 生成 UUID，用来给每个连接构造唯一的 identity
	"github.com/google/uuid"

	// 本项目自己的 config 包。
	// 【import 路径】streamforge/media-service 是 go.mod 里的 module 名，加上包路径就成完整 import。
	"streamforge/media-service/internal/config"
)

// Config 是 livekit 包自己视角下需要的配置项（从全局 config.Config 中抽取而来）。
//
// 【为什么单独抄一份】让本包不直接依赖 config.Config——保持解耦。
// 未来这个包完全可以从别的项目里复用，只需要它自己的这个小结构体就够了。
type Config struct {
	URL       string        // LiveKit 服务器地址（客户端将用它建立 WebRTC 连接）
	APIKey    string        // 相当于用户名
	APISecret string        // 相当于密码，绝不能出现在客户端
	TTL       time.Duration // Token 有效期
}

// TokenRequest 是"签一个 Token 的请求参数"。
type TokenRequest struct {
	RoomID   string // 我们业务侧的房间 ID
	RoomName string // 传给 LiveKit 的房间名（一般是 "streamforge-{RoomID}"，可选）
	UserID   int64  // 业务用户 ID
	Username string // 显示昵称
}

// TokenResult 是签发出来的 Token 及相关信息。
// 客户端会用其中的 URL + Token 连接 LiveKit。
type TokenResult struct {
	URL      string // LiveKit 服务器 URL
	RoomName string // 最终使用的房间名
	Identity string // 客户端在 LiveKit 侧的身份标识（对应 JWT 的 sub 字段）
	Token    string // 最终签发的 JWT 字符串
}

// Issuer 是 Token 签发器。持有配置和"时间/身份生成"策略。
//
// 【为什么把 now 和 newIdentity 做成字段】
// 和 state.RedisStore 一样：便于测试。
// 生产用 time.Now、生产用真实 uuid；测试可以打桩成固定值，让签出来的 Token 可复现。
type Issuer struct {
	cfg         Config
	now         func() time.Time            // 时间来源
	newIdentity func(userID int64) string   // identity 生成策略
}

// FromConfig 从整体的 config.Config 中抽取 livekit 相关部分。
// 让 main.go 里只写 livekit.NewIssuer(livekit.FromConfig(cfg)) 就够了，比较清爽。
func FromConfig(cfg config.Config) Config {
	return Config{
		URL:       cfg.LiveKitURL,
		APIKey:    cfg.LiveKitAPIKey,
		APISecret: cfg.LiveKitAPISecret,
		TTL:       cfg.LiveKitTokenTTL,
	}
}

// NewIssuer 用给定配置创建一个 Issuer。
// 默认策略：time.Now 作为时钟，NewIdentity 作为 identity 生成器。
func NewIssuer(cfg Config) *Issuer {
	return &Issuer{
		cfg:         cfg,
		now:         time.Now,
		newIdentity: NewIdentity,
	}
}

// SetIdentityGenerator 允许外部替换 identity 生成策略。
// 主要给测试代码用（"每次都返回 'user-42-abcd'"）。
func (i *Issuer) SetIdentityGenerator(generator func(userID int64) string) {
	if generator != nil {
		i.newIdentity = generator
	}
}

// Configured 判断"LiveKit 相关配置是否齐全"。
//
// 【为什么要这个方法】/health 端点会返回这个信息，方便运维查看依赖是否就绪。
// URL / Key / Secret 任何一个为空都视为未配置好。
func (i *Issuer) Configured() bool {
	return strings.TrimSpace(i.cfg.URL) != "" &&
		strings.TrimSpace(i.cfg.APIKey) != "" &&
		strings.TrimSpace(i.cfg.APISecret) != ""
}

// RoomName 根据业务房间 ID 生成 LiveKit 房间名。
// 前缀 "streamforge-" 是为了在 LiveKit 那边不会和别的应用重名。
func RoomName(roomID string) string {
	return "streamforge-" + strings.TrimSpace(roomID)
}

// NewIdentity 生成一个"LiveKit identity"。
//
// 【为什么加随机后缀】同一个用户可能在多个终端上开着（PC + 手机）同时进入房间，
// LiveKit 要求同一房间里 identity 唯一，否则后进的会把先进的踢下线。
// 这里在 userID 后面拼一段 uuid 前缀，保证唯一。
func NewIdentity(userID int64) string {
	// uuid.NewString() 类似 "550e8400-e29b-41d4-a716-446655440000"
	// strings.Split(...)[0] 取第一段 "550e8400"，足够短且够随机。
	return fmt.Sprintf("user-%d-%s", userID, strings.Split(uuid.NewString(), "-")[0])
}

// Issue 是这个包的核心方法：签发一个 LiveKit JWT Token。
//
// 【JWT 简介】JWT = header.payload.signature 三段用 . 拼接的字符串，每段是 base64url 编码。
// - header：{"alg":"HS256","typ":"JWT"} —— 声明签名算法和类型
// - payload（也叫 claims）：真实内容，比如 iss/sub/exp/自定义字段
// - signature：用 secret 对 base64url(header)+"."+base64url(payload) 做 HMAC-SHA256 得到
//
// 【LiveKit 特定要求】claims 里必须有 iss（=APIKey）、sub（=identity）、
// video.room（房间名）、video.roomJoin（是否允许加入房间）等字段。
func (i *Issuer) Issue(req TokenRequest) (TokenResult, error) {
	// 前置检查：配置不全就没法签，直接报错。
	if !i.Configured() {
		return TokenResult{}, errors.New("livekit config is incomplete")
	}

	// 如果调用方没传 RoomName，就用 RoomID 拼一个默认值。
	roomName := strings.TrimSpace(req.RoomName)
	if roomName == "" {
		roomName = RoomName(req.RoomID)
	}

	// 生成本次连接使用的 identity（每次都是新的）。
	identity := i.newIdentity(req.UserID)

	// 我们要把 roomId、userId、username 塞进 metadata，让客户端/其他成员能识别。
	// LiveKit 允许每个连接携带一段 metadata 字符串（服务端不解析，透传）。
	metadata, err := json.Marshal(map[string]interface{}{
		"roomId":   req.RoomID,
		"userId":   req.UserID,
		"username": req.Username,
	})
	if err != nil {
		return TokenResult{}, err
	}

	// 计算 nbf（Not Before）和 exp（Expiration）。
	issuedAt := i.now()
	ttl := i.cfg.TTL
	// TTL 未配置或非法时给一个默认 24 小时，避免签出永不过期的 Token。
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	// 构造 JWT claims。
	// 【注意】key 都是 JWT 标准/LiveKit 规范里规定的，不能随便改。
	claims := map[string]interface{}{
		"iss":      i.cfg.APIKey,          // Issuer：谁签的（我们的 API Key）
		"sub":      identity,              // Subject：本 Token 的主体（连接的身份）
		"name":     req.Username,          // 显示名字
		"metadata": string(metadata),      // 附带的自定义元数据（LiveKit 规定是字符串）
		"nbf":      issuedAt.Unix(),       // Not Before：从这个时间起 Token 才有效
		"exp":      issuedAt.Add(ttl).Unix(), // Expiration：到这个时间就失效
		"video": map[string]interface{}{
			"room":     roomName, // 只允许连接到指定房间
			"roomJoin": true,     // 允许加入这个房间
		},
	}

	// 用 HMAC-SHA256 签名。
	token, err := signJWT(claims, i.cfg.APISecret)
	if err != nil {
		return TokenResult{}, err
	}

	return TokenResult{
		URL:      i.cfg.URL,
		RoomName: roomName,
		Identity: identity,
		Token:    token,
	}, nil
}

// signJWT 手写实现 HS256 签名。
//
// 【流程】
//   1. 把 header 和 claims 分别 JSON 序列化后 base64url 编码
//   2. 拼接成 "headerB64.payloadB64" 的 unsigned 字符串
//   3. 用 secret 对 unsigned 做 HMAC-SHA256，得到二进制 signature
//   4. signature 也 base64url 编码后拼在末尾，得到最终 "header.payload.signature"
//
// 【为什么不直接用 jwt-go 之类的库】
//   - 少一个外部依赖；这段签名逻辑很短，标准库就能覆盖
//   - 学习目的：让流程完全透明
func signJWT(claims map[string]interface{}, secret string) (string, error) {
	// 固定的 header：告诉验证方"我用 HS256 算法"。
	header := map[string]interface{}{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	// 【base64.RawURLEncoding】RawURL 版本用 - 和 _ 代替 + 和 /，去掉尾部 = 填充，
	// 完全符合 JWT 规范。
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// "header.payload" 是要被签名的原文。
	unsigned := encodedHeader + "." + encodedClaims

	// hmac.New(hash, key) 返回一个 hash.Hash 接口的实现。
	mac := hmac.New(sha256.New, []byte(secret))
	// 【为什么忽略 Write 的返回值】hash.Hash.Write 永远不会返回 error。
	_, _ = mac.Write([]byte(unsigned))

	// mac.Sum(nil) 返回签名的二进制字节，我们再 base64url 编码它。
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	// 最终格式：header.payload.signature
	return unsigned + "." + signature, nil
}
