package middleware

import (
	"net/http"
	"strings"

	authpkg "ling-shu/internal/auth"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

const (
	UserIDKey   = "user_id"
	UsernameKey = "username"
)

func AuthRequired(tokens *authpkg.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokens == nil {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "auth is not configured")
			c.Abort()
			return
		}
		raw := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing bearer token")
			c.Abort()
			return
		}
		claims, err := tokens.Parse(strings.TrimSpace(raw[len("Bearer "):]))
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid bearer token")
			c.Abort()
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(UsernameKey, claims.Username)
		c.Next()
	}
}

func OptionalAuth(tokens *authpkg.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokens == nil {
			c.Next()
			return
		}
		raw := strings.TrimSpace(c.GetHeader("Authorization"))
		if raw == "" {
			c.Next()
			return
		}
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid authorization header")
			c.Abort()
			return
		}
		claims, err := tokens.Parse(strings.TrimSpace(raw[len("Bearer "):]))
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid bearer token")
			c.Abort()
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(UsernameKey, claims.Username)
		c.Next()
	}
}
