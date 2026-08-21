package automation

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger is a concurrency-safe timestamped append logger.
type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	file   *os.File
	prefix string
}

func NewLogger(w io.Writer) *Logger { return &Logger{out: w} }
func OpenLogger(path string) (*Logger, error) {
	if path == "" {
		return nil, ErrInvalidArgument
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if e != nil {
		return nil, e
	}
	return &Logger{out: f, file: f}, nil
}

// SetPrefix changes the prefix used by subsequent log entries.
func (l *Logger) SetPrefix(prefix string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.prefix = prefix
	l.mu.Unlock()
}

// Prefix returns the current log prefix.
func (l *Logger) Prefix() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prefix
}

func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		e := l.file.Close()
		l.file = nil
		l.out = nil
		return e
	}
	return nil
}
func (l *Logger) Printf(format string, args ...any) {
	_ = l.PrintfErr(format, args...)
}

// PrintfErr writes one complete log entry and reports writer failures.
func (l *Logger) PrintfErr(format string, args ...any) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.out == nil {
		return nil
	}
	prefix := l.prefix
	if prefix != "" {
		prefix += " "
	}
	_, err := fmt.Fprintf(l.out, "[%s] %s%s\n", time.Now().Format("15:04:05"), prefix, fmt.Sprintf(format, args...))
	return err
}
