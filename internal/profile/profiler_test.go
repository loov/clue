package profile

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNewProfiler_ReturnsProfilerForEitherMode(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		p := NewProfiler(true)
		if p == nil {
			t.Fatal("NewProfiler(true) returned nil")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		p := NewProfiler(false)
		if p == nil {
			t.Fatal("NewProfiler(false) returned nil")
		}
	})
}

func TestProfiler_RecordCompilationStoresEvents(t *testing.T) {
	p := NewProfiler(true)
	now := time.Now()

	// Record 3 compilations with different durations
	p.RecordCompilation("/path/to/foo.cpp", now, 100*time.Millisecond, 1)
	p.RecordCompilation("/path/to/bar.cpp", now, 200*time.Millisecond, 2)
	p.RecordCompilation("/path/to/baz.cpp", now, 300*time.Millisecond, 3)

	events := p.Events()
	if len(events) != 3 {
		t.Fatalf("Events() returned %d events, want 3", len(events))
	}

	// Verify source paths match
	sources := []string{events[0].Source, events[1].Source, events[2].Source}
	expectedSources := []string{"/path/to/foo.cpp", "/path/to/bar.cpp", "/path/to/baz.cpp"}
	for i, src := range sources {
		if src != expectedSources[i] {
			t.Errorf("events[%d].Source = %q, want %q", i, src, expectedSources[i])
		}
	}
}

func TestProfiler_RecordCompilationIgnoresEventsWhenDisabled(t *testing.T) {
	p := NewProfiler(false)
	now := time.Now()

	// Record 3 compilations on a disabled profiler
	p.RecordCompilation("/path/to/foo.cpp", now, 100*time.Millisecond, 1)
	p.RecordCompilation("/path/to/bar.cpp", now, 200*time.Millisecond, 2)
	p.RecordCompilation("/path/to/baz.cpp", now, 300*time.Millisecond, 3)

	events := p.Events()
	if len(events) != 0 {
		t.Fatalf("Events() on disabled profiler returned %d events, want 0", len(events))
	}
}

func TestProfiler_SlowestFilesSortsByDuration(t *testing.T) {
	p := NewProfiler(true)
	now := time.Now()

	// Record compilations with various durations: 100ms, 500ms, 200ms, 1s, 50ms
	p.RecordCompilation("/path/to/a.cpp", now, 100*time.Millisecond, 1)
	p.RecordCompilation("/path/to/b.cpp", now, 500*time.Millisecond, 2)
	p.RecordCompilation("/path/to/c.cpp", now, 200*time.Millisecond, 3)
	p.RecordCompilation("/path/to/d.cpp", now, 1*time.Second, 4)
	p.RecordCompilation("/path/to/e.cpp", now, 50*time.Millisecond, 5)

	slowest := p.SlowestFiles(3)
	if len(slowest) != 3 {
		t.Fatalf("SlowestFiles(3) returned %d events, want 3", len(slowest))
	}

	// Verify order: 1s, 500ms, 200ms (descending)
	expectedDurations := []time.Duration{1 * time.Second, 500 * time.Millisecond, 200 * time.Millisecond}
	for i, event := range slowest {
		if event.Duration != expectedDurations[i] {
			t.Errorf("slowest[%d].Duration = %v, want %v", i, event.Duration, expectedDurations[i])
		}
	}
}

func TestProfiler_SlowestFilesReturnsAllWhenLimitExceedsCount(t *testing.T) {
	p := NewProfiler(true)
	now := time.Now()

	// Record only 2 events
	p.RecordCompilation("/path/to/a.cpp", now, 100*time.Millisecond, 1)
	p.RecordCompilation("/path/to/b.cpp", now, 200*time.Millisecond, 2)

	// Request more than available
	slowest := p.SlowestFiles(10)
	if len(slowest) != 2 {
		t.Fatalf("SlowestFiles(10) with 2 events returned %d events, want 2", len(slowest))
	}
}

func TestProfiler_SlowestFilesReturnsNoneForNonpositiveLimit(t *testing.T) {
	p := NewProfiler(true)
	p.RecordCompilation("file.cpp", time.Now(), time.Second, 1)

	for _, n := range []int{0, -1} {
		if got := p.SlowestFiles(n); len(got) != 0 {
			t.Errorf("SlowestFiles(%d) returned %d events, want none", n, len(got))
		}
	}
}

func TestFormatDuration_UsesAdaptiveUnits(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{2300 * time.Millisecond, "[2.3s]"},
		{1000 * time.Millisecond, "[1.0s]"},
		{999 * time.Millisecond, "[999ms]"},
		{450 * time.Millisecond, "[450ms]"},
		{50 * time.Millisecond, "[50ms]"},
		{1 * time.Millisecond, "[1ms]"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestProfiler_PrintSlowestFilesIncludesNamesAndPercentages(t *testing.T) {
	p := NewProfiler(true)
	now := time.Now()

	// Record events
	p.RecordCompilation("/path/to/slow.cpp", now, 500*time.Millisecond, 1)
	p.RecordCompilation("/path/to/fast.cpp", now, 100*time.Millisecond, 2)
	p.RecordCompilation("/path/to/medium.cpp", now, 250*time.Millisecond, 3)

	var buf bytes.Buffer
	p.PrintSlowestFiles(3, &buf)
	output := buf.String()

	// Verify output contains expected header
	if !strings.Contains(output, "Slowest") {
		t.Errorf("output missing 'Slowest' header: %s", output)
	}

	// Verify output contains file names
	if !strings.Contains(output, "slow.cpp") {
		t.Errorf("output missing 'slow.cpp': %s", output)
	}
	if !strings.Contains(output, "fast.cpp") {
		t.Errorf("output missing 'fast.cpp': %s", output)
	}
	if !strings.Contains(output, "medium.cpp") {
		t.Errorf("output missing 'medium.cpp': %s", output)
	}

	// Verify output contains percentages
	if !strings.Contains(output, "%") {
		t.Errorf("output missing percentage symbol: %s", output)
	}
}

func TestProfiler_PrintSlowestFilesWritesNothingWhenEmpty(t *testing.T) {
	p := NewProfiler(true)
	// No events recorded

	var buf bytes.Buffer
	p.PrintSlowestFiles(3, &buf)
	output := buf.String()

	// Should produce no output for empty profiler
	if output != "" {
		t.Errorf("PrintSlowestFiles on empty profiler produced output: %q", output)
	}
}

func TestProfiler_TotalBuildTimeReportsElapsedTime(t *testing.T) {
	p := NewProfiler(true)
	p.Start()

	// Sleep briefly
	time.Sleep(15 * time.Millisecond)

	duration := p.TotalBuildTime()
	if duration < 10*time.Millisecond {
		t.Errorf("TotalBuildTime() = %v, want >= 10ms", duration)
	}
}
