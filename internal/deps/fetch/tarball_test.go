package fetch

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/loov/clue/internal/deps"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestTarballFetcherRejectsOversizedDownload(t *testing.T) {
	originalClient := tarballHTTPClient
	tarballHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(strings.NewReader("")),
			ContentLength: maxTarballBytes + 1,
		}, nil
	})}
	t.Cleanup(func() { tarballHTTPClient = originalClient })

	dep := deps.NewTarballDependency("large", "https://example.com/large.tar.gz", strings.Repeat("0", 64), "", nil)
	err := newTarballFetcher(false).fetch(t.Context(), dep, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "archive exceeds") {
		t.Fatalf("expected archive size error, got %v", err)
	}
}
