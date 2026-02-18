// Package breakpoint provides non-breaking breakpoint support for the Go agent.
package breakpoint

import "time"

// BreakpointInfo represents a registered breakpoint.
type BreakpointInfo struct {
	ID         string
	FilePath   string
	LineNumber int
	Condition  string
	MaxHits    int
	HitCount   int
	CreatedAt  time.Time
}
