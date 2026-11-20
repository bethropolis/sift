package compress

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// maxSummaryLines bounds every non-code summarizer so one large fixture cannot
// dominate a signature-mode document.
const maxSummaryLines = 240

// SummarizePath compresses non-code structured files that tree-sitter cannot
// parse: SQL retains DDL and drops bulk rows, JSON keeps its key/schema shape,
// YAML keeps its key hierarchy. It returns false when the file is unsupported
// or cannot be parsed, in which case callers keep the original content.
func SummarizePath(src []byte, path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sql":
		return summarizeSQL(src)
	case ".json":
		return summarizeJSON(src)
	case ".yaml", ".yml":
		return summarizeYAML(src)
	}
	return "", false
}

// summarizeSQL keeps schema statements and replaces bulk INSERT/COPY payloads
// with a count so migrations stay readable without carrying all fixture rows.
func summarizeSQL(src []byte) (string, bool) {
	statements := splitSQL(string(src))
	var b strings.Builder
	kept, skipped := 0, 0
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)
		switch {
		case hasSQLPrefix(upper, "INSERT"), hasSQLPrefix(upper, "COPY"),
			hasSQLPrefix(upper, "REPLACE"), hasSQLPrefix(upper, "VALUES"):
			skipped++
		default:
			b.WriteString(trimmed)
			b.WriteString(";\n\n")
			kept++
		}
	}
	if kept == 0 {
		return "", false
	}
	if skipped > 0 {
		fmt.Fprintf(&b, "-- ... %d bulk INSERT/COPY statements omitted by sift ...\n", skipped)
	}
	return strings.TrimRight(b.String(), "\n"), true
}

func hasSQLPrefix(upper, keyword string) bool {
	return strings.HasPrefix(upper, keyword+" ") || strings.HasPrefix(upper, keyword+"\n") ||
		strings.HasPrefix(upper, keyword+"\t") || strings.HasPrefix(upper, keyword+"(")
}

// splitSQL splits on semicolons outside single-quoted strings, double-quoted
// identifiers, line comments, and block comments.
func splitSQL(src string) []string {
	var out []string
	var b strings.Builder
	inSingle, inDouble, inLine, inBlock := false, false, false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		next := byte(0)
		if i+1 < len(src) {
			next = src[i+1]
		}
		switch {
		case inLine:
			b.WriteByte(c)
			if c == '\n' {
				inLine = false
			}
		case inBlock:
			b.WriteByte(c)
			if c == '*' && next == '/' {
				b.WriteByte(next)
				i++
				inBlock = false
			}
		case inSingle:
			b.WriteByte(c)
			if c == '\'' {
				if next == '\'' {
					b.WriteByte(next)
					i++
				} else {
					inSingle = false
				}
			}
		case inDouble:
			b.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
		default:
			switch {
			case c == '-' && next == '-':
				inLine = true
				b.WriteString("--")
				i++
			case c == '/' && next == '*':
				inBlock = true
				b.WriteString("/*")
				i++
			case c == '\'':
				inSingle = true
				b.WriteByte(c)
			case c == '"':
				inDouble = true
				b.WriteByte(c)
			case c == ';':
				out = append(out, b.String())
				b.Reset()
			default:
				b.WriteByte(c)
			}
		}
	}
	if strings.TrimSpace(b.String()) != "" {
		out = append(out, b.String())
	}
	return out
}

// summarizeJSON writes the structural skeleton of a JSON document: object
// keys, array lengths, and a shape for the first element. Invalid JSON is left
// alone so a broken spec is still visible in full.
func summarizeJSON(src []byte) (string, bool) {
	var value any
	if err := json.Unmarshal(src, &value); err != nil {
		return "", false
	}
	var b strings.Builder
	if !writeJSONShape(&b, value, 0) {
		return "", false
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if len(lines) > maxSummaryLines {
		lines = append(lines[:maxSummaryLines], "// ... remainder omitted by sift ...")
	}
	return strings.Join(lines, "\n"), true
}

func writeJSONShape(b *strings.Builder, value any, depth int) bool {
	indent := strings.Repeat("  ", depth)
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			b.WriteString("{}\n")
			return true
		}
		b.WriteString("{\n")
		for i, k := range keys {
			b.WriteString(indent + "  " + strconv.Quote(k) + ": ")
			writeJSONShape(b, v[k], depth+1)
			if i < len(keys)-1 {
				// writeJSONShape ends with a newline; commas stay explicit.
				trim := b.String()
				b.Reset()
				b.WriteString(strings.TrimSuffix(trim, "\n") + ",\n")
			}
		}
		b.WriteString(indent + "}\n")
		return true
	case []any:
		if len(v) == 0 {
			b.WriteString("[]\n")
			return true
		}
		fmt.Fprintf(b, "[ /* %d items */\n", len(v))
		b.WriteString(indent + "  ")
		writeJSONShape(b, v[0], depth+1)
		b.WriteString(indent + "]\n")
		return true
	case string:
		b.WriteString(strconv.Quote(truncateString(v, 60)) + "\n")
	case float64, bool, nil:
		fmt.Fprintf(b, "%v\n", v)
	default:
		b.WriteString("null\n")
	}
	return true
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// summarizeYAML keeps the key hierarchy and list item shape, dropping scalar
// payloads and collapsing repeated list entries. It is deliberately line-based
// so it never fails on a document larger than the JSON parser would accept.
func summarizeYAML(src []byte) (string, bool) {
	lines := strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	var out []string
	itemRuns := make(map[int]int)
	omitted := make(map[int]int)
	keys := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" || trimmed == "..." {
			continue
		}
		indentLen := len(line) - len(strings.TrimLeft(line, " "))
		indent := line[:indentLen]
		if strings.HasPrefix(trimmed, "- ") {
			itemRuns[indentLen]++
			if itemRuns[indentLen] > 3 {
				omitted[indentLen]++
				continue
			}
			body := strings.TrimSpace(trimmed[2:])
			if key, _, ok := splitYAMLKey(body); ok {
				out = append(out, indent+"- "+key+":")
				keys++
			} else {
				out = append(out, indent+"- "+truncateString(body, 60))
			}
			continue
		}
		itemRuns[indentLen] = 0
		if key, _, ok := splitYAMLKey(trimmed); ok {
			out = append(out, indent+key+":")
			keys++
		} else if len(out) > 0 && !strings.HasPrefix(out[len(out)-1], indent+"- ") {
			// Folded/continuation scalars: keep nothing.
			continue
		}
		if len(out) >= maxSummaryLines {
			break
		}
	}
	if keys == 0 {
		return "", false
	}
	var b strings.Builder
	b.WriteString(strings.Join(out, "\n"))
	b.WriteString("\n")
	var omittedTotal int
	for _, n := range omitted {
		omittedTotal += n
	}
	if omittedTotal > 0 {
		fmt.Fprintf(&b, "# ... %d list items omitted by sift ...\n", omittedTotal)
	}
	return strings.TrimRight(b.String(), "\n"), true
}

// splitYAMLKey separates "key: value" outside quotes. It returns false for
// bare scalars and list markers.
func splitYAMLKey(line string) (key, value string, ok bool) {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if inSingle || inDouble {
				continue
			}
			if i+1 < len(line) && line[i+1] != ' ' && line[i+1] != '\t' {
				continue
			}
			k := strings.TrimSpace(line[:i])
			if k == "" || strings.HasPrefix(k, "-") {
				return "", "", false
			}
			return k, strings.TrimSpace(line[i+1:]), true
		}
	}
	return "", "", false
}
