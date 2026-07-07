// Package config 负责加载 media-service 的配置。
//
// 【为什么单独一个包】
// 项目里几乎所有其他包（gateway、livekit、state...）都要用到"从哪里读 Redis"、
// "LiveKit 的密钥是什么"这类配置。把这些读取逻辑集中在一个地方，其他包就只需要
// 接收一个 Config 结构体，不用关心配置到底是从环境变量、.env 文件还是命令行读来的。
//
// 【配置来源的优先级】
//   1. 进程已经存在的环境变量（比如通过 docker run -e、systemd、K8s ConfigMap 注入的）
//   2. 项目根目录下的 .env 文件（本地开发方便，不用每次导环境变量）
//   3. 代码里写死的默认值（fallback）
// 也就是说：正式环境注入的环境变量永远优先，.env 只在环境变量缺失时兜底。
package config

import (
	"bufio"   // 缓冲式读取，用来一行一行读 .env 文件
	"os"      // 读环境变量、打开文件
	"strconv" // 字符串和数字互转（例如把 "8080" 变成 int 8080）
	"strings" // 字符串处理
	"time"    // 时间相关，用来表达"Token 有效期多少秒"
)

// Config 是整个 media-service 用到的所有配置项集合。
// 【Go 惯用法】结构体字段首字母大写表示"导出（public）"，其他包也能访问。
type Config struct {
	// ServerPort HTTP 服务监听的端口，例如 8080。
	ServerPort int

	// PublicWSBaseURL 前端最终能访问到的 WebSocket 基础 URL（含协议 + host + 端口）。
	// 之所以要单独拿出来，是因为服务可能跑在容器/反向代理后面，
	// 服务器内部监听 :8080，但对外可能是 wss://media.example.com。
	PublicWSBaseURL string

	// RedisAddr Redis 地址，格式为 "host:port"，例如 "127.0.0.1:6379"。
	RedisAddr string
	// RedisPassword Redis 密码（没有密码就是空字符串）。
	RedisPassword string
	// RedisDB Redis 的 DB 编号（0~15），不同微服务可以隔离在不同 DB。
	RedisDB int

	// LiveKitURL LiveKit 服务的地址，客户端要用它建立 WebRTC 连接。
	LiveKitURL string
	// LiveKitAPIKey LiveKit 分配给我们的 API Key（相当于身份）。
	LiveKitAPIKey string
	// LiveKitAPISecret 与 API Key 配对的密钥，用于签发 JWT Token（绝对不能泄漏到前端）。
	LiveKitAPISecret string
	// LiveKitTokenTTL Token 的有效期。到期后客户端需要重新申请。
	// 【类型说明】time.Duration 本质是 int64，单位是"纳秒"，
	// 但可以用 time.Second 等常量做乘法得到有意义的值。
	LiveKitTokenTTL time.Duration
}

// Load 读取并返回一份完整的 Config。
// 这是这个包对外的唯一入口，在 main.go 启动时被调用一次。
func Load() Config {
	// 先尝试加载 .env 文件里的键值对到进程环境变量里。
	// 用 _ 忽略错误，因为线上环境通常不需要 .env，文件不存在是正常的。
	_ = loadDotEnv(".env")

	// 读端口：如果 SERVER_PORT 环境变量没设或非法，就用默认 8080。
	serverPort := envInt("SERVER_PORT", 8080)

	// 读 WebSocket 主机名，用于拼默认的 PUBLIC_WS_BASE_URL。
	publicWSHost := env("PUBLIC_WS_HOST", "localhost")

	// 优先用完整的 PUBLIC_WS_BASE_URL；没配置就根据 host + port 自动拼一个 ws://... URL。
	publicWSBaseURL := env("PUBLIC_WS_BASE_URL", "ws://"+publicWSHost+":"+strconv.Itoa(serverPort))

	// Redis 主机和端口分开读，再拼成 "host:port" 形式。
	redisHost := env("REDIS_HOST", "localhost")
	redisPort := envInt("REDIS_PORT", 6379)

	// 【结构体字面量】Config{...} 在这里创建并返回一份 Config 值（不是指针）。
	// 因为 Config 里字段不多，值拷贝的开销可以忽略。
	return Config{
		ServerPort:       serverPort,
		PublicWSBaseURL:  publicWSBaseURL,
		RedisAddr:        redisHost + ":" + strconv.Itoa(redisPort),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"), // 密码可以为空，所以不给默认值
		RedisDB:          envInt("REDIS_DB", 0),
		LiveKitURL:       env("LIVEKIT_URL", "ws://localhost:7880"),
		LiveKitAPIKey:    env("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret: env("LIVEKIT_API_SECRET", "secret"),
		// 把"秒数"乘以 time.Second 得到 time.Duration。
		// 默认 86400 秒 = 24 小时。
		LiveKitTokenTTL: time.Duration(envInt("LIVEKIT_TOKEN_TTL_SECONDS", 86400)) * time.Second,
	}
}

// loadDotEnv 读取一个 .env 格式的文件，把里面的 KEY=VALUE 写到进程环境变量里。
// 只在环境变量还没有值时才写入（不覆盖真实环境）。
func loadDotEnv(path string) error {
	// os.Open 打开文件返回 *os.File 和 error。文件不存在时 err 不为 nil。
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	// 【Go 关键字 defer】在当前函数返回前一定会执行这行。
	// 这里保证文件句柄一定被关闭，避免资源泄漏。
	defer file.Close()

	// bufio.Scanner 提供"按行读取"的便捷 API。
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行和以 # 开头的注释行。
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 只按第一个 "=" 切一次。value 里如果本身就有 "=" 也不会被拆坏。
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// 允许 .env 里用引号包裹字符串，比如 SECRET="abc"，这里把引号去掉。
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		// 如果环境里已经有这个 key 且非空，就跳过——真实环境变量优先。
		if existing, ok := os.LookupEnv(key); ok && strings.TrimSpace(existing) != "" {
			continue
		}
		// 写回环境变量。忽略错误（在合规的 key/value 下几乎不会失败）。
		_ = os.Setenv(key, value)
	}
	// scanner.Err() 会返回读取过程中的 IO 错误（如果有的话）。
	return scanner.Err()
}

// env 读取一个字符串环境变量，若不存在或为空则返回 fallback。
// 【小工具函数】写在这里是因为多个字段都要用到"读取或用默认"的逻辑，抽出来避免重复。
func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

// envInt 读取一个整数环境变量。
// 与 env 类似，多做了一步：把字符串转成 int，转失败也当作没配置。
func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	// strconv.Atoi = ASCII to integer，把 "8080" 变成 8080。
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
