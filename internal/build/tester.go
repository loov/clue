package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// TestCase is one configured executable test invocation.
type TestCase struct {
	Name, Executable, WorkingDirectory string
	Args                               []string
	Environment                        map[string]string
}

// TestResult records one test invocation.
type TestResult struct {
	Name, Output string
	Duration     time.Duration
	Error        error
}

// TestSummary is the aggregate result of a test run.
type TestSummary struct {
	Results        []TestResult
	Passed, Failed int
	Duration       time.Duration
}

// RunTests executes tests in parallel and reports results in input order.
func RunTests(ctx context.Context, cases []TestCase, jobs int, verbosity Verbosity) TestSummary {
	start := time.Now()
	if jobs < 1 {
		jobs = 1
	}
	results := make([]TestResult, len(cases))
	tasks := make(chan int)
	var workers sync.WaitGroup
	for range min(jobs, len(cases)) {
		workers.Go(func() {
			for index := range tasks {
				results[index] = runTest(ctx, cases[index])
			}
		})
	}
	for index := range cases {
		tasks <- index
	}
	close(tasks)
	workers.Wait()

	summary := TestSummary{Results: results, Duration: time.Since(start)}
	for _, result := range results {
		if result.Error == nil {
			summary.Passed++
			if verbosity >= VerbosityNormal {
				fmt.Printf("[PASS] %s (%s)\n", result.Name, result.Duration.Round(time.Millisecond))
			}
			if verbosity == VerbosityVerbose && result.Output != "" {
				fmt.Print(result.Output)
			}
			continue
		}
		summary.Failed++
		fmt.Printf("[FAIL] %s (%s): %v\n", result.Name, result.Duration.Round(time.Millisecond), result.Error)
		if result.Output != "" {
			fmt.Print(result.Output)
			if !strings.HasSuffix(result.Output, "\n") {
				fmt.Println()
			}
		}
	}
	if verbosity >= VerbosityNormal || summary.Failed > 0 {
		fmt.Printf("Tests: %d passed, %d failed (%s)\n", summary.Passed, summary.Failed, summary.Duration.Round(time.Millisecond))
	}
	return summary
}

func runTest(ctx context.Context, test TestCase) TestResult {
	start := time.Now()
	command := exec.CommandContext(ctx, test.Executable, test.Args...)
	command.Dir = test.WorkingDirectory
	command.Env = os.Environ()
	for _, name := range sortedKeys(test.Environment) {
		command.Env = append(command.Env, name+"="+test.Environment[name])
	}
	output, err := command.CombinedOutput()
	return TestResult{Name: test.Name, Output: string(output), Duration: time.Since(start), Error: err}
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
