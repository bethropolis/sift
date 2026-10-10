export interface InstallMethod {
  id: string;
  name: string;
  command: string;
  note: string;
}

export const INSTALL_METHODS: InstallMethod[] = [
  {
    id: 'curl',
    name: 'macOS / Linux',
    command: 'curl -fsSL https://bethropolis.github.io/sift/install.sh | sh',
    note: 'Installs to ~/.local/bin. Set SIFT_INSTALL_DIR first to install somewhere else. If ~/.local/bin isn\'t on your PATH, add export PATH="$HOME/.local/bin:$PATH" to your shell profile.',
  },
  {
    id: 'homebrew',
    name: 'Homebrew',
    command: 'brew install bethropolis/tap/sift',
    note: 'Official Homebrew tap for macOS and Linux. Prebuilt binaries include tree-sitter signature mode.',
  },
  {
    id: 'scoop',
    name: 'Windows (Scoop)',
    command: 'scoop bucket add bethropolis https://github.com/bethropolis/scoop-bucket\nscoop install sift',
    note: 'Installs sift into your Scoop apps directory and shims the binary onto your PATH.',
  },
  {
    id: 'go',
    name: 'go install',
    command: 'go install github.com/bethropolis/sift/cmd/sift@latest',
    note: 'Builds a copy without the CGO-only signature engine unless your toolchain has cgo enabled. Prebuilt release binaries always include --mode signatures support.',
  },
  {
    id: 'source',
    name: 'Build from source',
    command: 'git clone https://github.com/bethropolis/sift.git\ncd sift\n./scripts/install.sh',
    note: 'Builds the checkout and installs to ~/.local/bin (override with SIFT_INSTALL_DIR). Or build manually with go build -o "$HOME/.local/bin/sift" ./cmd/sift.',
  },
];
