package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteTrace_ValidJSON(t *testing.T) {
	p := NewProfiler(true)
	p.Start()
	now := p.buildStart

	// Record 3 compilations with known values
	p.RecordCompilation("/path/to/foo.cpp", now.Add(10*time.Millisecond), 50*time.Millisecond, 1)
	p.RecordCompilation("/path/to/bar.cpp", now.Add(20*time.Millisecond), 100*time.Millisecond, 2)
	p.RecordCompilation("/path/to/baz.cpp", now.Add(30*time.Millisecond), 75*time.Millisecond, 3)

	tmpFile := filepath.Join(t.TempDir(), "trace.json")
	if err := p.WriteTrace(tmpFile); err != nil {
		t.Fatalf("WriteTrace failed: %v", err)
	}

	// Read and parse the JSON
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	var trace ChromeTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatalf("Failed to unmarshal JSON (invalid format): %v", err)
	}

	if len(trace.TraceEvents) != 3 {
		t.Errorf("TraceEvents has %d events, want 3", len(trace.TraceEvents))
	}
}

func TestWriteTrace_EventFields(t *testing.T) {
	p := NewProfiler(true)
	p.Start()
	now := p.buildStart

	// Record single known event
	sourcePath := "/path/to/myfile.cpp"
	threadID := 5
	p.RecordCompilation(sourcePath, now.Add(10*time.Millisecond), 50*time.Millisecond, threadID)

	tmpFile := filepath.Join(t.TempDir(), "trace.json")
	if err := p.WriteTrace(tmpFile); err != nil {
		t.Fatalf("WriteTrace failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	var trace ChromeTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if len(trace.TraceEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(trace.TraceEvents))
	}

	event := trace.TraceEvents[0]

	// Verify name is basename of source file
	if event.Name != "myfile.cpp" {
		t.Errorf("Name = %q, want %q", event.Name, "myfile.cpp")
	}

	// Verify category is "compile"
	if event.Category != "compile" {
		t.Errorf("Category = %q, want %q", event.Category, "compile")
	}

	// Verify phase is "X" (complete events)
	if event.Phase != "X" {
		t.Errorf("Phase = %q, want %q", event.Phase, "X")
	}

	// Verify ProcessID is 1
	if event.ProcessID != 1 {
		t.Errorf("ProcessID = %d, want 1", event.ProcessID)
	}

	// Verify ThreadID matches input
	if event.ThreadID != threadID {
		t.Errorf("ThreadID = %d, want %d", event.ThreadID, threadID)
	}

	// Verify Args contains "file" key with full path
	if event.Args == nil {
		t.Fatal("Args is nil")
	}
	fileArg, ok := event.Args["file"]
	if !ok {
		t.Error("Args missing 'file' key")
	} else if fileArg != sourcePath {
		t.Errorf("Args[file] = %q, want %q", fileArg, sourcePath)
	}
}

func TestWriteTrace_TimestampMicroseconds(t *testing.T) {
	p := NewProfiler(true)
	p.Start()
	now := p.buildStart

	// Record event: start=100ms from build start, duration=50ms
	startOffset := 100 * time.Millisecond
	duration := 50 * time.Millisecond
	p.RecordCompilation("/path/to/test.cpp", now.Add(startOffset), duration, 1)

	tmpFile := filepath.Join(t.TempDir(), "trace.json")
	if err := p.WriteTrace(tmpFile); err != nil {
		t.Fatalf("WriteTrace failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	var trace ChromeTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if len(trace.TraceEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(trace.TraceEvents))
	}

	event := trace.TraceEvents[0]

	// Verify ts field is in microseconds (100000 for 100ms)
	expectedTs := startOffset.Microseconds() // 100000
	if event.Timestamp != expectedTs {
		t.Errorf("Timestamp = %d, want %d (microseconds)", event.Timestamp, expectedTs)
	}

	// Verify dur field is in microseconds (50000 for 50ms)
	expectedDur := duration.Microseconds() // 50000
	if event.Duration != expectedDur {
		t.Errorf("Duration = %d, want %d (microseconds)", event.Duration, expectedDur)
	}
}

func TestWriteTrace_EmptyProfile(t *testing.T) {
	p := NewProfiler(true)
	// No events recorded

	tmpFile := filepath.Join(t.TempDir(), "trace.json")
	if err := p.WriteTrace(tmpFile); err != nil {
		t.Fatalf("WriteTrace failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Fatal("Trace file was not created")
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	var trace ChromeTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify empty traceEvents array
	if trace.TraceEvents == nil {
		t.Error("TraceEvents is nil, want empty slice")
	} else if len(trace.TraceEvents) != 0 {
		t.Errorf("TraceEvents has %d events, want 0", len(trace.TraceEvents))
	}
}

func TestWriteTrace_FileError(t *testing.T) {
	p := NewProfiler(true)

	// Call WriteTrace with invalid path
	invalidPath := "/nonexistent/dir/profile.json"
	err := p.WriteTrace(invalidPath)

	if err == nil {
		t.Fatal("Expected error for invalid path, got nil")
	}

	// Verify error message contains context
	if !strings.Contains(err.Error(), "failed to create trace file") {
		t.Errorf("Error message should contain 'failed to create trace file': %v", err)
	}
}

func TestChromeTrace_PrettyPrint(t *testing.T) {
	p := NewProfiler(true)
	p.Start()
	now := p.buildStart

	p.RecordCompilation("/path/to/test.cpp", now.Add(10*time.Millisecond), 50*time.Millisecond, 1)

	tmpFile := filepath.Join(t.TempDir(), "trace.json")
	if err := p.WriteTrace(tmpFile); err != nil {
		t.Fatalf("WriteTrace failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	content := string(data)

	// Verify contains newlines (pretty printed)
	if !strings.Contains(content, "\n") {
		t.Error("Output should contain newlines (pretty printed)")
	}

	// Verify contains indentation (spaces at start of lines)
	lines := strings.Split(content, "\n")
	hasIndentation := false
	for _, line := range lines {
		if strings.HasPrefix(line, "  ") {
			hasIndentation = true
			break
		}
	}
	if !hasIndentation {
		t.Error("Output should contain indentation (not minified)")
	}
}
