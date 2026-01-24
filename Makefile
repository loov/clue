.PHONY: lint vet staticcheck revive golangci-lint test build clean all

# Default target
all: lint test build

# Run all linters
lint: vet staticcheck revive golangci-lint

# Individual linter targets
vet:
	go vet ./...

staticcheck:
	staticcheck ./...

revive:
	revive ./...

golangci-lint:
	golangci-lint run ./...

fmt:
	gofumpt -l -w .

modernize:
	go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...

# Other common targets
test:
	go test ./...

build:
	go build -o clue .

clean:
	rm -rf .build clue
