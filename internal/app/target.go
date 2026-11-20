package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Target is a file reference with an optional symbol or 1-based line range.
type Target struct {
	Path   string
	Symbol string
	Start  int
	End    int
}

// IsFocused reports whether the target selects a symbol or line range.
func (t Target) IsFocused() bool { return t.Symbol != "" || (t.Start > 0 && t.End >= t.Start) }

var lineRangeSuffix = regexp.MustCompile(`:(\d+)-(\d+)$`)

// ParseTarget parses "path", "path#Symbol", "path#Type.Method", or
// "path:start-end". A Windows drive letter (C:\...) is not mistaken for a
// range separator.
func ParseTarget(spec string) (Target, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return Target{}, fmt.Errorf("focus: empty target")
	}
	driveEnd := 0
	if len(spec) >= 3 && spec[1] == ':' && isDriveLetter(spec[0]) && (spec[2] == '\\' || spec[2] == '/') {
		driveEnd = 2
	}

	// Symbol form wins over a range when both appear after the drive prefix.
	if idx := strings.LastIndex(spec[driveEnd:], "#"); idx >= 0 {
		hash := driveEnd + idx
		path := spec[:hash]
		symbol := strings.TrimSpace(spec[hash+1:])
		if path == "" || symbol == "" {
			return Target{}, fmt.Errorf("focus: invalid symbol target %q", spec)
		}
		return Target{Path: path, Symbol: symbol}, nil
	}

	if m := lineRangeSuffix.FindStringSubmatch(spec[driveEnd:]); m != nil {
		start, err1 := strconv.Atoi(m[1])
		end, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil || start < 1 || end < start {
			return Target{}, fmt.Errorf("focus: invalid line range %q", m[0])
		}
		path := spec[:driveEnd+strings.LastIndex(spec[driveEnd:], ":")]
		if path == "" {
			return Target{}, fmt.Errorf("focus: missing path in %q", spec)
		}
		return Target{Path: path, Start: start, End: end}, nil
	}

	return Target{Path: spec}, nil
}

func isDriveLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
