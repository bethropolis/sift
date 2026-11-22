// Package qr is a small, dependency-free QR encoder used by `sift serve
// --pair`. The low-level coding and GF(256) tables are adapted from the
// BSD-licensed rsc.io/qr package (Copyright 2010-2011 The Go Authors); only
// the encoder is retained, and the terminal renderer is local to sift.
package qr

import (
	"strings"

	"github.com/bethropolis/sift/internal/serve/qr/coding"
)

// Level is a QR error-correction level.
type Level int

const (
	L Level = iota
	M
	Q
	H
)

// Code is a rendered QR matrix. Bitmap holds one byte per module (1 = dark).
type Code struct {
	Bitmap []byte
	Size   int
	Stride int
}

// Black reports whether the module at (x, y) is dark.
func (c *Code) Black(x, y int) bool {
	if x < 0 || y < 0 || x >= c.Size || y >= c.Size {
		return false
	}
	return c.Bitmap[y*c.Stride+x/8]&(1<<uint(7-x&7)) != 0
}

// Encode returns a QR code for text at the requested error-correction level.
func Encode(text string, level Level) (*Code, error) {
	var enc coding.Encoding
	switch {
	case coding.Num(text).Check() == nil:
		enc = coding.Num(text)
	case coding.Alpha(text).Check() == nil:
		enc = coding.Alpha(text)
	default:
		enc = coding.String(text)
	}
	l := coding.Level(level)
	var v coding.Version
	for v = coding.MinVersion; ; v++ {
		if v > coding.MaxVersion {
			return nil, errTooLong
		}
		if enc.Bits(v) <= v.DataBytes(l)*8 {
			break
		}
	}
	p, err := coding.NewPlan(v, l, 0)
	if err != nil {
		return nil, err
	}
	cc, err := p.Encode(enc)
	if err != nil {
		return nil, err
	}
	return &Code{Bitmap: cc.Bitmap, Size: cc.Size, Stride: cc.Stride}, nil
}

// errTooLong is returned when the text does not fit in any QR version.
var errTooLong = qrError("text too long to encode as QR")

type qrError string

func (e qrError) Error() string { return string(e) }

// Terminal renders the code as compact Unicode half blocks with a two-module
// quiet zone. The token URL is short enough that this stays scannable in a
// typical terminal.
func (c *Code) Terminal() string {
	const quiet = 2
	row := func(y int) bool { return y >= 0 && y < c.Size }
	dark := func(x, y int) bool {
		if x < 0 || x >= c.Size || y < 0 || y >= c.Size {
			return false
		}
		return c.Black(x, y)
	}
	var b strings.Builder
	for y := -quiet; y < c.Size+quiet; y += 2 {
		for x := -quiet; x < c.Size+quiet; x++ {
			top := row(y) && dark(x, y)
			bottom := row(y+1) && dark(x, y+1)
			switch {
			case top && bottom:
				b.WriteRune('█')
			case top:
				b.WriteRune('▀')
			case bottom:
				b.WriteRune('▄')
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
