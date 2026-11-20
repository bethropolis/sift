//go:build !cgo

package mcp

import "testing"

func requireCompression(t *testing.T) { t.Skip("signature compression requires cgo") }
