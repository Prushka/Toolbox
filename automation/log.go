package automation

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
)

// NewLogger returns a structured, timestamped Zerolog logger. SyncWriter makes
// logging safe even when the supplied writer is not concurrency-safe.
func NewLogger(writer io.Writer) zerolog.Logger {
	if writer == nil {
		return zerolog.Nop()
	}
	return zerolog.New(zerolog.SyncWriter(writer)).With().Timestamp().Logger()
}

// FileLogger owns an append-opened log file and embeds its Zerolog logger.
// Call Close after all goroutines using the logger have stopped.
type FileLogger struct {
	zerolog.Logger
	file      *lockedFile
	closeOnce sync.Once
	closeErr  error
}

type lockedFile struct {
	mu       sync.Mutex
	file     *os.File
	writeErr error
}

func (file *lockedFile) Write(payload []byte) (int, error) {
	file.mu.Lock()
	defer file.mu.Unlock()
	if file.file == nil {
		return 0, os.ErrClosed
	}
	written, err := file.file.Write(payload)
	if err != nil {
		file.writeErr = errors.Join(file.writeErr, err)
	} else if written != len(payload) {
		err = io.ErrShortWrite
		file.writeErr = errors.Join(file.writeErr, err)
	}
	return written, err
}

func (file *lockedFile) Close() error {
	file.mu.Lock()
	defer file.mu.Unlock()
	if file.file == nil {
		return nil
	}
	syncErr := file.file.Sync()
	closeErr := file.file.Close()
	writeErr := file.writeErr
	file.file = nil
	return errors.Join(writeErr, syncErr, closeErr)
}

// OpenLogger opens path for structured JSON logging. Every event includes a
// timestamp and is appended with Zerolog's allocation-conscious JSON encoder.
func OpenLogger(path string) (*FileLogger, error) {
	if path == "" {
		return nil, ErrInvalidArgument
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	locked := &lockedFile{file: file}
	return &FileLogger{
		Logger: zerolog.New(locked).With().Timestamp().Logger(),
		file:   locked,
	}, nil
}

// Close flushes the append file and closes it. It is idempotent.
func (logger *FileLogger) Close() error {
	if logger == nil || logger.file == nil {
		return nil
	}
	logger.closeOnce.Do(func() {
		logger.closeErr = logger.file.Close()
	})
	return logger.closeErr
}
