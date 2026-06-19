package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyFileWriterRotatesByDate(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewDailyFileWriter(dir, "ling-shu")
	if err != nil {
		t.Fatalf("new daily writer: %v", err)
	}
	defer func() {
		if writer.file != nil {
			_ = writer.file.Close()
		}
	}()

	now := time.Date(2026, 6, 18, 23, 59, 0, 0, time.Local)
	writer.now = func() time.Time { return now }
	if _, err := writer.Write([]byte("first\n")); err != nil {
		t.Fatalf("write first log: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := writer.Write([]byte("second\n")); err != nil {
		t.Fatalf("write second log: %v", err)
	}
	if err := writer.Sync(); err != nil {
		t.Fatalf("sync writer: %v", err)
	}

	firstPath := filepath.Join(dir, "ling-shu-2026-06-18.log")
	secondPath := filepath.Join(dir, "ling-shu-2026-06-19.log")
	currentPath := filepath.Join(dir, "ling-shu.log")
	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first day log: %v", err)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second day log: %v", err)
	}
	if strings.TrimSpace(string(first)) != "first" {
		t.Fatalf("unexpected first day content: %q", string(first))
	}
	if strings.TrimSpace(string(second)) != "second" {
		t.Fatalf("unexpected second day content: %q", string(second))
	}
	target, err := os.Readlink(currentPath)
	if err != nil {
		t.Fatalf("read current log symlink: %v", err)
	}
	if target != filepath.Base(secondPath) {
		t.Fatalf("unexpected current log target: %q", target)
	}
}
