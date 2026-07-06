package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Config struct {
	MediaInstanceID string
	ServerPort      int
	PublicHTTPHost  string
	PublicWSBaseURL string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	RTCPortRange    string
	ICEStunURLs     []string
}

func Load() Config {
	_ = loadDotEnv(".env")

	serverPort := envInt("SERVER_PORT", 8080)
	instanceID := strings.TrimSpace(os.Getenv("MEDIA_INSTANCE_ID"))
	if instanceID == "" {
		if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
			instanceID = hostname
		} else {
			instanceID = "media-" + uuid.NewString()
		}
	}

	publicWSHost := env("PUBLIC_WS_HOST", "localhost")
	publicWSBaseURL := env("PUBLIC_WS_BASE_URL", "ws://"+publicWSHost+":"+strconv.Itoa(serverPort))
	redisHost := env("REDIS_HOST", "localhost")
	redisPort := envInt("REDIS_PORT", 6379)

	return Config{
		MediaInstanceID: instanceID,
		ServerPort:      serverPort,
		PublicHTTPHost:  env("PUBLIC_HTTP_HOST", "localhost"),
		PublicWSBaseURL: publicWSBaseURL,
		RedisAddr:       redisHost + ":" + strconv.Itoa(redisPort),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		RedisDB:         envInt("REDIS_DB", 0),
		RTCPortRange:    env("RTC_UDP_PORT_MIN", "30000") + "-" + env("RTC_UDP_PORT_MAX", "30100"),
		ICEStunURLs:     splitCSV(env("ICE_STUN_URLS", "stun:stun.l.google.com:19302")),
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if existing, ok := os.LookupEnv(key); ok && strings.TrimSpace(existing) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
