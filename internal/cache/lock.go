package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type Lock struct {
	store Store
	key   string
	token string
}

func TryLock(ctx context.Context, store Store, prefix string, rawKey string, ttl time.Duration) (*Lock, bool, error) {
	if store == nil {
		return nil, true, nil
	}
	key := BuildKey(prefix, rawKey)
	token := time.Now().UTC().Format(time.RFC3339Nano)
	ok, err := store.SetNX(ctx, key, token, ttl)
	if err != nil || !ok {
		return nil, ok, err
	}
	return &Lock{store: store, key: key, token: token}, true, nil
}

func (l *Lock) Release(ctx context.Context) error {
	if l == nil || l.store == nil || l.key == "" {
		return nil
	}
	value, ok, err := l.store.Get(ctx, l.key)
	if err != nil || !ok || value != l.token {
		return err
	}
	return l.store.Del(ctx, l.key)
}

func BuildKey(prefix string, parts ...string) string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), ":")
	if cleanPrefix == "" {
		cleanPrefix = "ling-shu"
	}
	segments := []string{cleanPrefix}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, stableDigest(part))
	}
	return strings.Join(segments, ":")
}

func stableDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:12])
}
