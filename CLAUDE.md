# Clue - C++ Build System

A C++ build system written in Go using CUE for configuration.

## Project Structure

- `main.go` - CLI entry point
- `internal/` - Core packages (build, config, deps, generate, graph, errors)
- `schema/` - CUE schema definitions
- `testdata/` - Test fixtures

## Development Commands

### Linting

Run all linters before committing:

```bash
make lint
```

Individual linters:
- `make vet` - Go vet (correctness)
- `make staticcheck` - Static analysis
- `make revive` - Style and conventions
- `make golangci-lint` - Combined linter suite

### Formatting

```bash
make fmt        # Format code with gofumpt
make modernize  # Apply Go modernization fixes
```

### Testing

```bash
make test
# or
go test ./...
```

### Building

```bash
make build
# or
go build -o clue .
```

## Verification Requirements

When completing tasks, ALWAYS run:

1. `make lint` - All linters must pass
2. `make test` - All tests must pass

Do not consider a task complete until both commands succeed.
