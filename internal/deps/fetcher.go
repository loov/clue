package deps

import "context"

// Fetcher defines the interface for fetching dependencies
type Fetcher interface {
	// Fetch retrieves the dependency and stores it at targetPath
	Fetch(ctx context.Context, dep Dependency, targetPath string) error
}
