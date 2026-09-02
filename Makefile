.PHONY: lint fmt-check vet staticcheck revive golangci-lint fmt modernize test build clean all

GOFUMPT = go run mvdan.cc/gofumpt@v0.9.2
STATICCHECK = go run honnef.co/go/tools/cmd/staticcheck@v0.8.1
REVIVE = go run github.com/mgechev/revive@v1.15.0
GOLANGCI_LINT = go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
MODERNIZE = go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@v0.38.0

# Default target
all: lint test build

# Run all linters
lint: fmt-check vet staticcheck revive golangci-lint

fmt-check:
	@test -z "$$($(GOFUMPT) -l .)" || { $(GOFUMPT) -d .; exit 1; }

# Individual linter targets
vet:
	go vet ./...

staticcheck:
	$(STATICCHECK) ./...

revive:
	$(REVIVE) -config revive.toml ./...

golangci-lint:
	$(GOLANGCI_LINT) run ./...

fmt:
	$(GOFUMPT) -l -w .

modernize:
	$(MODERNIZE) -fix ./...

# Other common targets
test:
	go test ./...

build:
	go build -o clue .

clean:
	rm -rf .build clue
