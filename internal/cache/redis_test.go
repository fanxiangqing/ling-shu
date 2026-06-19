package cache

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"
)

func TestReadRedisValue(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{name: "simple string", payload: "+PONG\r\n", want: "PONG"},
		{name: "integer", payload: ":42\r\n", want: "42"},
		{name: "bulk string", payload: "$5\r\nhello\r\n", want: "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := readRedisValue(bufio.NewReader(strings.NewReader(tt.payload)))
			if err != nil {
				t.Fatalf("read redis value: %v", err)
			}
			if value.stringValue() != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, value.stringValue())
			}
		})
	}
}

func TestBuildKeyUsesStableDigest(t *testing.T) {
	key1 := BuildKey("ling-shu:query:lock", "select * from users")
	key2 := BuildKey("ling-shu:query:lock", "select * from users")
	if key1 != key2 {
		t.Fatalf("expected stable key, got %q and %q", key1, key2)
	}
	if strings.Contains(key1, "select") {
		t.Fatalf("expected key to avoid raw SQL, got %q", key1)
	}
}

func TestTryLockWithoutStoreDoesNotBlock(t *testing.T) {
	lock, ok, err := TryLock(context.Background(), nil, "test", "key", time.Second)
	if err != nil {
		t.Fatalf("try lock: %v", err)
	}
	if !ok {
		t.Fatal("expected disabled lock to pass")
	}
	if lock != nil {
		t.Fatalf("expected no lock when store is nil")
	}
}
