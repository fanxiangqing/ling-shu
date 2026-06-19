package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DailyFileWriter struct {
	mu       sync.Mutex
	dir      string
	name     string
	now      func() time.Time
	date     string
	file     *os.File
	openFile func(string) (*os.File, error)
}

func NewDailyFileWriter(dir string, name string) (*DailyFileWriter, error) {
	writer := &DailyFileWriter{
		dir:      strings.TrimSpace(dir),
		name:     strings.TrimSpace(name),
		now:      time.Now,
		openFile: openLogFile,
	}
	if writer.dir == "" {
		writer.dir = "logs"
	}
	if writer.name == "" {
		writer.name = "app"
	}
	if err := os.MkdirAll(writer.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	return writer, nil
}

func (w *DailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateLocked(w.now()); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *DailyFileWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *DailyFileWriter) rotateLocked(now time.Time) error {
	date := now.Format("2006-01-02")
	if w.file != nil && w.date == date {
		return nil
	}

	if w.file != nil {
		_ = w.file.Sync()
		_ = w.file.Close()
		w.file = nil
	}

	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.name, date))
	file, err := w.openFile(path)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.updateCurrentLink(path)
	w.file = file
	w.date = date
	return nil
}

func (w *DailyFileWriter) updateCurrentLink(path string) {
	link := filepath.Join(w.dir, fmt.Sprintf("%s.log", w.name))
	info, err := os.Lstat(link)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return
		}
		if removeErr := os.Remove(link); removeErr != nil {
			return
		}
	} else if !os.IsNotExist(err) {
		return
	}
	_ = os.Symlink(filepath.Base(path), link)
}

func openLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}
