//go:build !cgo

package picker

import "testing"

func requireCompression(t *testing.T) { t.Skip("signature compression requires cgo") }
