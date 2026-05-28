binary := "transcript-exporter"

# List available recipes
default:
    @just --list

# --- Build ---

# Build the CLI binary into the repo root
[group('build')]
build:
    go build -o {{binary}} ./cmd/transcript-exporter

# Run the exporter (pass flags through, e.g. `just run --project thane-ai-agent --target ./out --dry-run`)
[group('build')]
run *ARGS:
    go run ./cmd/transcript-exporter {{ARGS}}

# Remove build artifacts
[group('build')]
clean:
    rm -f {{binary}}

# --- Test / lint ---

# Run tests with the race detector
[group('test')]
test:
    go test -race ./...

# Report uncovered code as an HTML profile
[group('test')]
cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out

# Format all Go sources in place
[group('test')]
fmt:
    gofmt -w .

# Fail if any file is not gofmt-clean
[group('test')]
fmt-check:
    @test -z "$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# go vet
[group('test')]
vet:
    go vet ./...

# golangci-lint (v2 config in .golangci.yml)
[group('test')]
lint:
    golangci-lint run ./...

# Fail if go.mod/go.sum are not tidy
[group('test')]
mod-tidy-check:
    go mod tidy
    @test -z "$(git status --porcelain go.mod go.sum)" || (echo "go.mod/go.sum not tidy — run 'go mod tidy'" && git --no-pager diff go.mod go.sum && exit 1)

# Full local validation gate — run this before every push
[group('test')]
ci: fmt-check vet mod-tidy-check lint test
