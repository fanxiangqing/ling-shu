package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RateLimitStore interface {
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

type RateLimitConfig struct {
	Enabled  bool
	Requests int
	Window   time.Duration
	Prefix   string
}

func RateLimit(store RateLimitStore, cfg RateLimitConfig, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled || store == nil || cfg.Requests <= 0 || cfg.Window <= 0 {
			c.Next()
			return
		}

		key := rateLimitKey(cfg.Prefix, c)
		count, err := store.Increment(c.Request.Context(), key, cfg.Window)
		if err != nil {
			if logger != nil {
				logger.Warn("rate limit check failed",
					zap.String("path", c.Request.URL.Path),
					zap.String("key", key),
					zap.Error(err),
				)
			}
			c.Next()
			return
		}

		limit := int64(cfg.Requests)
		c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.Requests))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(maxInt64(limit-count, 0), 10))
		c.Header("X-RateLimit-Window", cfg.Window.String())
		if count > limit {
			response.Error(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

func rateLimitKey(prefix string, c *gin.Context) string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), ":")
	if cleanPrefix == "" {
		cleanPrefix = "ling-shu:rate"
	}
	if value, ok := c.Get(UserIDKey); ok {
		if userID, ok := value.(uint64); ok && userID > 0 {
			return fmt.Sprintf("%s:user:%d", cleanPrefix, userID)
		}
	}
	return fmt.Sprintf("%s:ip:%s", cleanPrefix, digest(c.ClientIP()))
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
