//go:build cgo

package app

import "testing"

// requireCompression is a no-op in cgo builds, where tree-sitter signatures
// are available.
func requireCompression(t *testing.T) {}
