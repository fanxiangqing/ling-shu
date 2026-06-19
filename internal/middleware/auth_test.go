package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authpkg "ling-shu/internal/auth"

	"github.com/gin-gonic/gin"
)

func TestOptionalAuthSetsUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := authpkg.NewTokenManager("secret", time.Hour)
	token, _, err := tokens.Generate(7, "alice")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	engine := gin.New()
	engine.Use(OptionalAuth(tokens))
	engine.GET("/me", func(c *gin.Context) {
		value, _ := c.Get(UserIDKey)
		c.JSON(http.StatusOK, gin.H{"user_id": value})
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != `{"user_id":7}` {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestOptionalAuthAllowsAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(OptionalAuth(authpkg.NewTokenManager("secret", time.Hour)))
	engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}
