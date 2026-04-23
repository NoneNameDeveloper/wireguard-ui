// Package logger wraps slog with a file+stderr multiplexer and simple rotation.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const maxLogSize = 10 * 1024 * 1024

var (
	mu sync.Mutex
	lg *slog.Logger
)

func Init(logPath string, level slog.Level) error {
	mu.Lock()
	defer mu.Unlock()

	writers := []io.Writer{os.Stderr}
	if logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
			if st, err := os.Stat(logPath); err == nil && st.Size() > maxLogSize {
				_ = os.Truncate(logPath, 0)
			}
			if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
				writers = append(writers, f)
			} else {
				fmt.Fprintf(os.Stderr, "wgtray: cannot open log %s: %v\n", logPath, err)
			}
		}
	}
	lg = slog.New(slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level}))
	slog.SetDefault(lg)
	return nil
}

func L() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()
	if lg == nil {
		lg = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return lg
}

func ParseLevel(s string) slog.Level {
	switch s {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN", "warning":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
