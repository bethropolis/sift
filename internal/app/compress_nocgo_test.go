//go:build !cgo

package app

import "testing"

// requireCompression skips tests that assert tree-sitter signature output when
// the build has no cgo engine.
func requireCompression(t *testing.T) {
	t.Helper()
	t.Skip("signature compression requires cgo")
}
