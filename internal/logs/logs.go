// Package logs keeps a bounded, in-memory log of application events at
// three levels (info, warning, error) so the web UI can surface them in
// its Logs section. Entries never contain secrets: every registered
// secret is masked to asterisks (actual length preserved) before the
// message is stored.
package logs

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Level is the severity of a log entry.
type Level string

const (
	Info    Level = "info"
	Warning Level = "warning"
	Error   Level = "error"
)

// Entry is a single timestamped log line.
type Entry struct {
	Time    time.Time `json:"time"`
	Level   Level     `json:"level"`
	Message string    `json:"message"`
}

// Logger is a concurrency-safe ring buffer of log entries.
type Logger struct {
	mu      sync.RWMutex
	entries []Entry
	limit   int
	secrets []string
}

// New creates a Logger retaining at most limit entries (minimum 100).
func New(limit int) *Logger {
	if limit < 100 {
		limit = 100
	}
	return &Logger{limit: limit}
}

// RegisterSecret adds a value that must never appear verbatim in a
// stored message; it is masked to asterisks of the same length.
func (l *Logger) RegisterSecret(values ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || len(v) < 4 {
			continue
		}
		dup := false
		for _, existing := range l.secrets {
			if existing == v {
				dup = true
				break
			}
		}
		if !dup {
			l.secrets = append(l.secrets, v)
		}
	}
	// Longest first so overlapping secrets mask correctly.
	sort.SliceStable(l.secrets, func(i, j int) bool { return len(l.secrets[i]) > len(l.secrets[j]) })
}

// Mask returns s with every registered secret replaced by asterisks
// of the secret's actual length.
func (l *Logger) Mask(s string) string {
	l.mu.RLock()
	secrets := l.secrets
	l.mu.RUnlock()
	for _, secret := range secrets {
		s = strings.ReplaceAll(s, secret, strings.Repeat("*", len(secret)))
	}
	return s
}

func (l *Logger) add(level Level, msg string) {
	msg = l.Mask(msg)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, Entry{Time: time.Now(), Level: level, Message: msg})
	if len(l.entries) > l.limit {
		l.entries = l.entries[len(l.entries)-l.limit:]
	}
}

// Info records an informational message.
func (l *Logger) Info(format string, args ...any) { l.add(Info, sprintf(format, args...)) }

// Warn records a warning message.
func (l *Logger) Warn(format string, args ...any) { l.add(Warning, sprintf(format, args...)) }

// Error records an error message.
func (l *Logger) Error(format string, args ...any) { l.add(Error, sprintf(format, args...)) }

// All returns entries newest first, optionally filtered by level
// (empty level returns all levels).
func (l *Logger) All(level Level) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, 0, len(l.entries))
	for i := len(l.entries) - 1; i >= 0; i-- {
		if level != "" && l.entries[i].Level != level {
			continue
		}
		out = append(out, l.entries[i])
	}
	return out
}

// Clear drops all entries.
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Default is the process-wide logger used by the server and main.
var Default = New(1000)

// Package-level helpers delegate to Default.
func LogInfo(format string, args ...any)  { Default.Info(format, args...) }
func LogWarn(format string, args ...any)  { Default.Warn(format, args...) }
func LogError(format string, args ...any) { Default.Error(format, args...) }
