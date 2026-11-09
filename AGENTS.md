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

Follow the Go project style:

```
internal/plan: compile clang module interfaces as c++-module

Clang writes the BMI that -fmodule-output names only for an input it
compiles as a module interface, which it tells from the extension. An
interface in a .cc or .cpp file, such as libstdc++'s std.cc, produced
no BMI, so its importers failed to find it.
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
- The subject completes the sentence "This change modifies clue to ..."
- No period at the end
- Keep the whole line under 72 characters

### Body

- Leave a blank line after the subject, then the body. Wrap it at 72 columns.
- The body explains why: what was wrong, what the change does about it,
  anything a reviewer would otherwise have to dig for. The diff already
  shows how.
- Use full sentences. Plain text, no markdown headings, no bullet soup.
- Skip the body only when the subject says everything.
- Put references like `Fixes #12` or `Updates #12` at the end.
- No `Co-Authored-By` or other attribution trailers.

### Unslop

Before committing, reread the message and fix these:

- Puffery and AI words. "enhance", "robust", "seamless", "leverage",
  "crucial", "comprehensive", "streamline", "ensure", "delve". Say what
  happened: "add", "fix", "remove", "use".
- Selling the change. "greatly improves", "significantly faster". Give the
  number or drop the adverb.
- Restating the diff. "Update custom.go and plan.go" or a list of every
  touched function. Say why instead.
- Filler. "This commit", "In order to", "It is worth noting that". Start
  with the point.
- Padding the list. Don't force three bullets when there is one reason.
  Don't add "Also improved readability" for changes nobody asked about.
- Punctuation tells. No em dashes, no bold, no emoji, straight quotes only.
  Avoid colons mid-sentence, the one after the scope is the exception.
- Passive voice. "the cache is invalidated" becomes "reload clears the
  cache".
- Hedging. "should probably help with" becomes what it does, or say plainly
  that it is untested.

Last check: if the sentence would fit unchanged in another project's log,
it says nothing about this one. Rewrite or cut it.

### Examples

```
internal/build: add caching for compiled objects
internal/deps: fix tarball extraction on Windows
internal/build, internal/config: refactor error types
all: modernize to Go 1.23 conventions
```

## Workflow

Commit each change as soon as it is implemented and passes the
verification below, one commit per logical change.

## Verification Requirements

When completing tasks, ALWAYS run:

1. `make lint` - All linters must pass
2. `make test` - All tests must pass

Do not consider a task complete until both commands succeed.
