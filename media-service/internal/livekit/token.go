package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"streamforge/media-service/internal/config"
)

type Config struct {
	URL       string
	APIKey    string
	APISecret string
	TTL       time.Duration
}

type TokenRequest struct {
	RoomID   string
	RoomName string
	UserID   int64
	Username string
}

type TokenResult struct {
	URL      string
	RoomName string
	Identity string
	Token    string
}

type Issuer struct {
	cfg         Config
	now         func() time.Time
	newIdentity func(userID int64) string
}

func FromConfig(cfg config.Config) Config {
	return Config{
		URL:       cfg.LiveKitURL,
		APIKey:    cfg.LiveKitAPIKey,
		APISecret: cfg.LiveKitAPISecret,
		TTL:       cfg.LiveKitTokenTTL,
	}
}

func NewIssuer(cfg Config) *Issuer {
	return &Issuer{
		cfg:         cfg,
		now:         time.Now,
		newIdentity: NewIdentity,
	}
}

func (i *Issuer) SetIdentityGenerator(generator func(userID int64) string) {
	if generator != nil {
		i.newIdentity = generator
	}
}

func (i *Issuer) Configured() bool {
	return strings.TrimSpace(i.cfg.URL) != "" && strings.TrimSpace(i.cfg.APIKey) != "" && strings.TrimSpace(i.cfg.APISecret) != ""
}

func RoomName(roomID string) string {
	return "streamforge-" + strings.TrimSpace(roomID)
}

func NewIdentity(userID int64) string {
	return fmt.Sprintf("user-%d-%s", userID, strings.Split(uuid.NewString(), "-")[0])
}

func (i *Issuer) Issue(req TokenRequest) (TokenResult, error) {
	if !i.Configured() {
		return TokenResult{}, errors.New("livekit config is incomplete")
	}
	roomName := strings.TrimSpace(req.RoomName)
	if roomName == "" {
		roomName = RoomName(req.RoomID)
	}
	identity := i.newIdentity(req.UserID)
	metadata, err := json.Marshal(map[string]interface{}{
		"roomId":   req.RoomID,
		"userId":   req.UserID,
		"username": req.Username,
	})
	if err != nil {
		return TokenResult{}, err
	}
	issuedAt := i.now()
	ttl := i.cfg.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	claims := map[string]interface{}{
		"iss":      i.cfg.APIKey,
		"sub":      identity,
		"name":     req.Username,
		"metadata": string(metadata),
		"nbf":      issuedAt.Unix(),
		"exp":      issuedAt.Add(ttl).Unix(),
		"video": map[string]interface{}{
			"room":     roomName,
			"roomJoin": true,
		},
	}
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

func signJWT(claims map[string]interface{}, secret string) (string, error) {
	header := map[string]interface{}{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := encodedHeader + "." + encodedClaims
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + signature, nil
}
