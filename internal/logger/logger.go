package logger

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Logger struct {
	mu sync.Mutex
}

type LogEntry struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

func New() *Logger {
	return &Logger{}
}

func (l *Logger) log(level, message string, data map[string]any) {
	entry := LogEntry{
		Time:    time.Now().UTC().Format(time.RFC3339),
		Level:   level,
		Message: message,
		Data:    data,
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_ = json.NewEncoder(os.Stdout).Encode(entry)
}

func (l *Logger) Info(message string, data map[string]any) {
	l.log("info", message, data)
}

func (l *Logger) Error(message string, data map[string]any) {
	l.log("error", message, data)
}

func (l *Logger) Warn(message string, data map[string]any) {
	l.log("warn", message, data)
}
