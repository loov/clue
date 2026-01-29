package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ChromeEvent represents a single event in Chrome Trace format
type ChromeEvent struct {
	Name      string                 `json:"name"`           // Display name (file basename)
	Category  string                 `json:"cat,omitempty"`  // Event category ("compile")
	Phase     string                 `json:"ph"`             // Phase type ("X" for complete events)
	Timestamp int64                  `json:"ts"`             // Microseconds from trace start
	Duration  int64                  `json:"dur"`            // Duration in microseconds
	ProcessID int                    `json:"pid"`            // Process ID (always 1)
	ThreadID  int                    `json:"tid"`            // Thread/worker ID
	Args      map[string]interface{} `json:"args,omitempty"` // Additional arguments
}

// ChromeTrace represents the complete Chrome Trace JSON structure
type ChromeTrace struct {
	TraceEvents []ChromeEvent `json:"traceEvents"`
}

// buildChromeTrace converts profiler events to Chrome Trace format
func (p *Profiler) buildChromeTrace() ChromeTrace {
	p.mu.Lock()
	defer p.mu.Unlock()

	trace := ChromeTrace{
		TraceEvents: make([]ChromeEvent, 0, len(p.events)),
	}

	for _, event := range p.events {
		chromeEvent := ChromeEvent{
			Name:      filepath.Base(event.Source),
			Category:  "compile",
			Phase:     "X",
			Timestamp: event.StartTime.Sub(p.buildStart).Microseconds(),
			Duration:  event.Duration.Microseconds(),
			ProcessID: 1,
			ThreadID:  event.ThreadID,
			Args: map[string]interface{}{
				"file": event.Source,
			},
		}
		trace.TraceEvents = append(trace.TraceEvents, chromeEvent)
	}

	return trace
}

// WriteTrace exports the profiler data to a Chrome Trace JSON file
func (p *Profiler) WriteTrace(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create trace file: %w", err)
	}
	defer f.Close()

	trace := p.buildChromeTrace()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(trace); err != nil {
		return fmt.Errorf("failed to encode trace: %w", err)
	}

	return nil
}
