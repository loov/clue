// Package profile collects and reports build timing information for compilation units.
//
// The profile package tracks compilation events with microsecond precision, identifies
// the slowest compilation units, and exports Chrome Trace format for visualization in
// chrome://tracing or similar tools.
//
// Key types:
//   - Profiler: Thread-safe collector for compilation timing events
//   - CompileEvent: A single compilation event with source file, start time, duration, and thread ID
//
// The Chrome Trace export uses the JSON format compatible with Chrome's built-in tracing
// viewer. Events include begin/end markers for each compilation, with thread IDs for
// parallel build visualization.
//
// Example:
//
//	p := profile.NewProfiler(true)
//	p.Start()
//	p.RecordCompilation(sourceFile, startTime, duration, workerID)
//	p.PrintSlowestFiles(10, os.Stdout)
//	profile.WriteChromeTrace(p, outputFile)
package profile
