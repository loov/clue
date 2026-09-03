package profile

import (
	"cmp"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// CompileEvent represents a single compilation timing event
type CompileEvent struct {
	Source    string        // Source file path
	StartTime time.Time     // Absolute start time of compilation
	Duration  time.Duration // Time taken for this compilation
	ThreadID  int           // Worker goroutine ID for Chrome Trace
}

// Profiler collects and aggregates build timing information
type Profiler struct {
	mu         sync.Mutex
	enabled    bool
	buildStart time.Time
	events     []CompileEvent
}

// NewProfiler creates a new Profiler instance
func NewProfiler(enabled bool) *Profiler {
	return &Profiler{
		enabled:    enabled,
		buildStart: time.Now(),
		events:     make([]CompileEvent, 0),
	}
}

// Start records the build start time
func (p *Profiler) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buildStart = time.Now()
}

// RecordCompilation records a compilation event
func (p *Profiler) RecordCompilation(source string, start time.Time, duration time.Duration, threadID int) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, CompileEvent{
		Source:    source,
		StartTime: start,
		Duration:  duration,
		ThreadID:  threadID,
	})
}

// GetEvents returns a copy of all recorded events
func (p *Profiler) GetEvents() []CompileEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]CompileEvent, len(p.events))
	copy(result, p.events)
	return result
}

// GetSlowestFiles returns the n slowest compilation events sorted by duration descending
func (p *Profiler) GetSlowestFiles(n int) []CompileEvent {
	if n <= 0 {
		return nil
	}
	events := p.GetEvents()
	slices.SortFunc(events, func(a, b CompileEvent) int {
		return cmp.Compare(b.Duration, a.Duration)
	})
	if n > len(events) {
		n = len(events)
	}
	return events[:n]
}

// TotalBuildTime returns the total elapsed time since build start
func (p *Profiler) TotalBuildTime() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return time.Since(p.buildStart)
}

// formatDuration formats a duration with adaptive precision
func formatDuration(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("[%.1fs]", d.Seconds())
	}
	return fmt.Sprintf("[%dms]", d.Milliseconds())
}

// PrintSlowestFiles prints the n slowest compilation units to the given writer
func (p *Profiler) PrintSlowestFiles(n int, w io.Writer) {
	slowest := p.GetSlowestFiles(n)
	if len(slowest) == 0 {
		return
	}

	// Calculate total compilation time for percentage
	var totalCompileTime time.Duration
	for _, e := range p.GetEvents() {
		totalCompileTime += e.Duration
	}

	_, _ = fmt.Fprintf(w, "\nSlowest %d compilation units:\n", len(slowest))
	for _, e := range slowest {
		percentage := float64(e.Duration) / float64(totalCompileTime) * 100
		_, _ = fmt.Fprintf(w, "  %s  %s  (%.1f%%)\n",
			formatDuration(e.Duration),
			filepath.Base(e.Source),
			percentage)
	}
}
