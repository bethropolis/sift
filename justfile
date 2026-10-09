# Sift: LLM context packer

binary := 'bin/sift'
pkg := './cmd/sift'
# Version stamped into the binary; falls back to "dev" outside a git checkout.
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

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

# Build the sift binary into {{ binary }}, stamped with the git version
build:
  mkdir -p bin
  go build -ldflags "-X github.com/bethropolis/sift/internal/config.Version={{version}}" -o {{ binary }} {{ pkg }}

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

# Preview the current directory with an instruction prompt
prompt:
  go run {{ pkg }} dump . --prompt "Review this codebase for race conditions."

# Scan the current directory as XML with token counts
run-xml:
  go run {{ pkg }} dump . --style xml

# Dump only files changed since git HEAD (uncommitted work)
diff:
  go run {{ pkg }} diff

# Dump only changes since the last recorded dump (incremental)
delta:
  go run {{ pkg }} delta

# Dump the changes as a raw unified diff patch
delta-patch:
  go run {{ pkg }} delta --patch

# Dump signature-only summaries of the current directory
sig:
  go run {{ pkg }} dump . --mode signatures --style xml

# Build the serve web UI (live API; gzip-precompressed into web/dist).
# Isolated mock UI work: cd web && VITE_MOCK=true bun run dev
web:
  sh scripts/build-web.sh

# Regenerate the web theme data + CSS from internal/theme (single source).
gen-themes:
  go run ./internal/theme/genweb

# Dev server for the web UI with /api proxied to a local `sift serve` (live)
web-dev:
  cd web && VITE_MOCK=false bun run dev

# Check the web UI size budgets (initial JS <60KB gz, total <150KB gz)
web-budget:
  cd web && bun scripts/check-budget.ts

# Fresh web UI build, then local install (binary to ~/.local/bin,
# override with SIFT_INSTALL_DIR=...). install.sh reuses the just-built UI.
install: web
  bash scripts/install.sh

# Remove the local install: binary + generated completions
# (--purge also removes the config/state directory)
uninstall args="":
  bash scripts/uninstall.sh {{ args }}

# Sync docs/*.md into a site source dir as the /docs/ collection (default: site/)
docs-site dest="site":
  bash scripts/sync-docs.sh {{ dest }}

# Serve the site locally, mirroring the Pages build (sync docs + installer)
site-serve:
  rm -rf .site-preview
  mkdir -p .site-preview
  cp -R site/. .site-preview/
  rm -rf .site-preview/vendor .site-preview/_site
  bash scripts/sync-docs.sh .site-preview
  cp scripts/install-online.sh .site-preview/install.sh
  chmod 755 .site-preview/install.sh
  cd .site-preview && BUNDLE_GEMFILE="$PWD/Gemfile" BUNDLE_PATH="{{ justfile_directory() }}/site/vendor/bundle" bundle exec jekyll serve --host 0.0.0.0 --port 3000

# Remove the built binary
clean:
  rm -rf bin
