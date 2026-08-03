---
layout: home
title: Sift
---

# Sift

Sift turns a codebase into focused, LLM-friendly context. Browse files in an
interactive picker, choose full or signature representations, and generate a
clean context document.

## Install

### macOS or Linux

```sh
curl -fsSL https://bethropolis.github.io/sift/install.sh | sh
```

The installer supports `SIFT_INSTALL_DIR` when you want a different location:

```sh
SIFT_INSTALL_DIR="$HOME/bin" curl -fsSL https://bethropolis.github.io/sift/install.sh | sh
```

### Homebrew

```sh
brew install bethropolis/tap/sift
```

### Windows

```powershell
scoop bucket add bethropolis https://github.com/bethropolis/scoop-bucket
scoop install sift
```

## Local development

From a checkout, run `./scripts/install.sh`. The script builds the current
source and installs it to `~/.local/bin` by default.
