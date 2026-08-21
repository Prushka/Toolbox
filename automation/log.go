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
	Prefix string
}

func NewLogger(w io.Writer) *Logger { return &Logger{out: w} }
func OpenLogger(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if e != nil {
		return nil, e
	}
	return &Logger{out: f, file: f}, nil
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
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.out == nil {
		return
	}
	prefix := l.Prefix
	if prefix != "" {
		prefix += " "
	}
	fmt.Fprintf(l.out, "[%s] %s%s\n", time.Now().Format("15:04:05"), prefix, fmt.Sprintf(format, args...))
}
