// Package engine provides core functionality for managing Java/Maven projects.
package engine

import (
	"sync"
	"time"
)

// LogEntry represents a single log entry for a project.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"` // info, warn, error, debug, build
	Message   string `json:"message"`
}

// LogBuffer provides a per-project ring buffer for log entries.
type LogBuffer struct {
	mu        sync.RWMutex
	max       int
	buffers   map[string][]LogEntry
	positions map[string]int // current write position
}

// NewLogBuffer creates a LogBuffer with the specified max entries per project.
func NewLogBuffer(maxEntries int) *LogBuffer {
	if maxEntries <= 0 {
		maxEntries = 5000
	}
	return &LogBuffer{
		max:       maxEntries,
		buffers:   make(map[string][]LogEntry),
		positions: make(map[string]int),
	}
}

// Append adds a log entry for the given project.
func (b *LogBuffer) Append(projectID string, entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	buf, ok := b.buffers[projectID]
	if !ok {
		buf = make([]LogEntry, b.max)
		b.buffers[projectID] = buf
		b.positions[projectID] = 0
	}

	pos := b.positions[projectID]
	buf[pos] = entry
	b.positions[projectID] = (pos + 1) % b.max
}

// Get returns all log entries for the given project, in chronological order.
func (b *LogBuffer) Get(projectID string) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	buf, ok := b.buffers[projectID]
	if !ok {
		return []LogEntry{}
	}

	pos := b.positions[projectID]
	result := make([]LogEntry, 0, b.max)

	// Start from pos (oldest) and wrap around
	for i := 0; i < b.max; i++ {
		idx := (pos + i) % b.max
		if buf[idx].Timestamp == "" && buf[idx].Message == "" {
			continue // skip empty slots
		}
		result = append(result, buf[idx])
	}

	return result
}

// Clear removes all log entries for the given project.
func (b *LogBuffer) Clear(projectID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.buffers, projectID)
	delete(b.positions, projectID)
}

// Remove removes all log entries for the given project (alias for Clear).
func (b *LogBuffer) Remove(projectID string) {
	b.Clear(projectID)
}

// NewLogEntry creates a LogEntry with the current time formatted as HH:MM:SS.mmm.
func NewLogEntry(level, message string) LogEntry {
	now := time.Now()
	ts := now.Format("15:04:05.000")
	return LogEntry{
		Timestamp: ts,
		Level:     level,
		Message:   message,
	}
}
