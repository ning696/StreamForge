package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort       int
	PublicWSBaseURL  string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string
	LiveKitTokenTTL  time.Duration
}

func Load() Config {
	_ = loadDotEnv(".env")

	serverPort := envInt("SERVER_PORT", 8080)
	publicWSHost := env("PUBLIC_WS_HOST", "localhost")
	publicWSBaseURL := env("PUBLIC_WS_BASE_URL", "ws://"+publicWSHost+":"+strconv.Itoa(serverPort))
	redisHost := env("REDIS_HOST", "localhost")
	redisPort := envInt("REDIS_PORT", 6379)

	return Config{
		ServerPort:       serverPort,
		PublicWSBaseURL:  publicWSBaseURL,
		RedisAddr:        redisHost + ":" + strconv.Itoa(redisPort),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          envInt("REDIS_DB", 0),
		LiveKitURL:       env("LIVEKIT_URL", "ws://localhost:7880"),
		LiveKitAPIKey:    env("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret: env("LIVEKIT_API_SECRET", "secret"),
		LiveKitTokenTTL:  time.Duration(envInt("LIVEKIT_TOKEN_TTL_SECONDS", 86400)) * time.Second,
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
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
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
