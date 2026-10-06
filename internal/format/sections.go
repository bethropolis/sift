package format

import "io"

// Section is one file's byte range inside a rendered document. Start is the
// offset of the file block's first byte, End one past its last byte, so
// doc[Start:End] is exactly that file's rendered block. Tokens is the file's
// token count at render time.
type Section struct {
	Path   string
	Tokens int
	Start  int
	End    int
}

// countWriter tallies bytes passing through to w so renderers can record
// per-file output ranges. Only used when section collection is enabled.
type countWriter struct {
	w io.Writer
	n int
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += n
	return n, err
}

// sectionOut wraps w for byte counting when out is non-nil, returning the
// writer to use plus the counter (nil when disabled, keeping the hot path
// allocation-free).
func sectionOut(w io.Writer, out *[]Section) (io.Writer, *countWriter) {
	if out == nil {
		return w, nil
	}
	cw := &countWriter{w: w}
	return cw, cw
}

// recordSection appends a file's output range. A nil counter means
// collection is disabled and the call is a no-op.
func recordSection(out *[]Section, cw *countWriter, path string, tokens, start int) {
	if out == nil || cw == nil {
		return
	}
	*out = append(*out, Section{Path: path, Tokens: tokens, Start: start, End: cw.n})
}
