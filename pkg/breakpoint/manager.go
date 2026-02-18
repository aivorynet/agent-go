package breakpoint

import (
	"encoding/json"
	"log"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const maxCapturesPerSecond = 50

// Sender is the interface for sending breakpoint hits to the backend.
type Sender interface {
	SendBreakpointHit(breakpointID string, payload map[string]interface{})
}

// Manager manages non-breaking breakpoints for the Go agent.
// Provides a manual API: developers place breakpoint.Hit("id") calls
// at locations of interest, and the backend enables/disables them remotely.
type Manager struct {
	debug       bool
	sender      Sender
	breakpoints map[string]*BreakpointInfo
	mu          sync.RWMutex

	captureCount       int
	captureWindowStart time.Time
}

// NewManager creates a new breakpoint manager.
func NewManager(debug bool, sender Sender) *Manager {
	return &Manager{
		debug:              debug,
		sender:             sender,
		breakpoints:        make(map[string]*BreakpointInfo),
		captureWindowStart: time.Now(),
	}
}

// SetBreakpoint registers a breakpoint.
func (m *Manager) SetBreakpoint(id, filePath string, lineNumber int, condition string, maxHits int) {
	if maxHits < 1 {
		maxHits = 1
	}
	if maxHits > 50 {
		maxHits = 50
	}

	m.mu.Lock()
	m.breakpoints[id] = &BreakpointInfo{
		ID:         id,
		FilePath:   filePath,
		LineNumber: lineNumber,
		Condition:  condition,
		MaxHits:    maxHits,
		CreatedAt:  time.Now(),
	}
	m.mu.Unlock()

	if m.debug {
		log.Printf("[AIVory Monitor] Breakpoint set: %s at %s:%d", id, filePath, lineNumber)
	}
}

// RemoveBreakpoint removes a breakpoint.
func (m *Manager) RemoveBreakpoint(id string) {
	m.mu.Lock()
	delete(m.breakpoints, id)
	m.mu.Unlock()

	if m.debug {
		log.Printf("[AIVory Monitor] Breakpoint removed: %s", id)
	}
}

// Hit triggers a breakpoint capture.
// Only captures if the breakpoint ID is registered and active.
func (m *Manager) Hit(id string) {
	m.mu.RLock()
	bp, exists := m.breakpoints[id]
	m.mu.RUnlock()

	if !exists {
		return
	}

	if bp.HitCount >= bp.MaxHits {
		return
	}

	if !m.rateLimitOk() {
		return
	}

	m.mu.Lock()
	bp.HitCount++
	hitCount := bp.HitCount
	m.mu.Unlock()

	if m.debug {
		log.Printf("[AIVory Monitor] Breakpoint hit: %s", id)
	}

	stackTrace := m.buildStackTrace()

	payload := map[string]interface{}{
		"captured_at": time.Now().UnixMilli(),
		"file_path":   bp.FilePath,
		"line_number": bp.LineNumber,
		"stack_trace": stackTrace,
		"hit_count":   hitCount,
	}

	m.sender.SendBreakpointHit(bp.ID, payload)
}

// HandleCommand handles a breakpoint command from the backend.
func (m *Manager) HandleCommand(command string, payload interface{}) {
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		// Try to unmarshal from JSON bytes
		if data, ok := payload.(json.RawMessage); ok {
			var pm map[string]interface{}
			if err := json.Unmarshal(data, &pm); err == nil {
				payloadMap = pm
			}
		}
		if payloadMap == nil {
			return
		}
	}

	switch command {
	case "set":
		id, _ := payloadMap["id"].(string)
		filePath, _ := payloadMap["file_path"].(string)
		if filePath == "" {
			filePath, _ = payloadMap["file"].(string)
		}
		lineNumber := 0
		if ln, ok := payloadMap["line_number"].(float64); ok {
			lineNumber = int(ln)
		} else if ln, ok := payloadMap["line"].(float64); ok {
			lineNumber = int(ln)
		}
		condition, _ := payloadMap["condition"].(string)
		maxHits := 1
		if mh, ok := payloadMap["max_hits"].(float64); ok {
			maxHits = int(mh)
		}

		m.SetBreakpoint(id, filePath, lineNumber, condition, maxHits)

	case "remove":
		id, _ := payloadMap["id"].(string)
		m.RemoveBreakpoint(id)
	}
}

func (m *Manager) rateLimitOk() bool {
	now := time.Now()
	if now.Sub(m.captureWindowStart) >= time.Second {
		m.captureCount = 0
		m.captureWindowStart = now
	}

	if m.captureCount >= maxCapturesPerSecond {
		if m.debug {
			log.Println("[AIVory Monitor] Rate limit reached, skipping capture")
		}
		return false
	}

	m.captureCount++
	return true
}

func (m *Manager) buildStackTrace() []map[string]interface{} {
	var pcs [50]uintptr
	// Skip 3 frames: runtime.Callers, buildStackTrace, Hit
	n := runtime.Callers(3, pcs[:])

	var frames []map[string]interface{}
	callFrames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := callFrames.Next()

		fileName := ""
		if frame.File != "" {
			fileName = filepath.Base(frame.File)
		}

		frames = append(frames, map[string]interface{}{
			"method_name": frame.Function,
			"file_path":   frame.File,
			"file_name":   fileName,
			"line_number": frame.Line,
			"is_native":   frame.File == "",
		})

		if !more {
			break
		}
	}

	return frames
}
