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

## Commit Messages

Use Go standard commit message format:

```
<scope>: <subject>
```

### Scope Rules

- **Single package**: Use the package path
  ```
  internal/build: add parallel compilation
  internal/config: fix variant merging
  ```

- **Two packages**: Use comma separator
  ```
  internal/build, internal/deps: fix error handling
  ```

- **Three+ packages or root files**: Use `all`
  ```
  all: update error handling across packages
  all: fix linter issues
  ```

- **Other scopes**: `.devcontainer`, `testdata`, `schema`, `docs`

### Subject Guidelines

- Use lowercase, imperative mood ("add", "fix", "update", not "Added", "Fixes")
- No period at the end
- Keep under 72 characters

### Examples

```
internal/build: add caching for compiled objects
internal/deps: fix tarball extraction on Windows
internal/build, internal/config: refactor error types
all: modernize to Go 1.23 conventions
```

## Verification Requirements

When completing tasks, ALWAYS run:

1. `make lint` - All linters must pass
2. `make test` - All tests must pass

Do not consider a task complete until both commands succeed.
