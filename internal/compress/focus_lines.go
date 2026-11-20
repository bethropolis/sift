package compress

import (
	"fmt"
	"strings"
)

// LineSlice returns 1-based inclusive lines start..end with omission markers
// around the slice. It is used for line-range focus and as the no-cgo
// fallback.
func LineSlice(src []byte, start, end int) ([]byte, error) {
	if start < 1 || end < start {
		return nil, fmt.Errorf("focus: invalid line range %d-%d", start, end)
	}
	lines := strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	total := len(lines)
	if start > total {
		return nil, fmt.Errorf("focus: line %d is past the end of the file (%d lines)", start, total)
	}
	if end > total {
		end = total
	}

	var b strings.Builder
	if start > 1 {
		fmt.Fprintf(&b, "// ... lines 1-%d omitted by sift focus ...\n", start-1)
	}
	b.WriteString(strings.Join(lines[start-1:end], "\n"))
	b.WriteByte('\n')
	if end < total {
		fmt.Fprintf(&b, "// ... lines %d-%d omitted by sift focus ...\n", end+1, total)
	}
	return []byte(b.String()), nil
}
