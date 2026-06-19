package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

func (m *TokenManager) Generate(userID uint64, username string) (string, time.Time, error) {
	if len(m.secret) == 0 || userID == 0 || strings.TrimSpace(username) == "" {
		return "", time.Time{}, ErrInvalidToken
	}
	now := time.Now()
	expiresAt := now.Add(m.ttl)
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := Claims{
		UserID:   userID,
		Username: strings.TrimSpace(username),
		IssuedAt: now.Unix(),
		Expires:  expiresAt.Unix(),
	}
	headerPart, err := encodeJSON(header)
	if err != nil {
		return "", time.Time{}, err
	}
	claimsPart, err := encodeJSON(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	signingInput := headerPart + "." + claimsPart
	signature := sign(signingInput, m.secret)
	return signingInput + "." + signature, expiresAt, nil
}

func (m *TokenManager) Parse(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(m.secret) == 0 {
		return nil, ErrInvalidToken
	}
	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(signingInput, m.secret)), []byte(parts[2])) {
		return nil, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.UserID == 0 || claims.Username == "" {
		return nil, ErrInvalidToken
	}
	if claims.Expires <= time.Now().Unix() {
		return nil, ErrExpiredToken
	}
	return &claims, nil
}

func encodeJSON(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(content), nil
}

func sign(input string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
