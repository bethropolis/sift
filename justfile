# Dir-Dumper: LLM context packer

binary := 'bin/dumper'
pkg := './cmd/dumper'

# List all recipes
default:
  @just --list

# Format check (fail on any unformatted file)
fmt:
  gofmt -l .
  test -z "$(gofmt -l .)"

# Format and rewrite files in place
fmt-write:
  gofmt -w .

# Run go vet
vet:
  go vet ./...

# Build the dumper binary into {{ binary }}
build:
  mkdir -p bin
  go build -o {{ binary }} {{ pkg }}

# Build, vet, and format-check
check: fmt vet build

# Run all tests
test:
  go test ./...

# Run all tests with the race detector
test-race:
  go test -race ./...

# Write the default codebase.md for the current directory
dump:
  go run {{ pkg }} dump .

# Preview the current directory on stdout (no file)
run:
  go run {{ pkg }} dump . --output -

# Launch the interactive file picker
pick:
  go run {{ pkg }} pick .

# Scan the current directory as XML with token counts
run-xml:
  go run {{ pkg }} dump . --style xml

# Dump only files changed since git HEAD (uncommitted work)
diff:
  go run {{ pkg }} diff

# Dump signature-only summaries of the current directory
sig:
  go run {{ pkg }} dump . --mode signatures --style xml

# Remove the built binary
clean:
  rm -rf bin
