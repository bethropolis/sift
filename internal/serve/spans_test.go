package serve

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/bethropolis/sift/internal/highlight"
)

// TestHighlightKindNames pins the span wire vocabulary: every TokenKind maps
// to a lowercase wire name (client-safe), with plain reserved for fallback.
func TestHighlightKindNames(t *testing.T) {
	wire := regexp.MustCompile(`^[a-z]+$`)
	seen := map[string]highlight.TokenKind{}
	for k := highlight.TokenPlain; k <= highlight.TokenDiffHunk; k++ {
		name := highlightKindName(k)
		if name == "" {
			t.Fatalf("TokenKind %d has no wire name", uint8(k))
		}
		if !wire.MatchString(name) {
			t.Fatalf("TokenKind %d wire name %q is not lowercase alpha", uint8(k), name)
		}
		if prev, dup := seen[name]; dup && name != "plain" {
			t.Fatalf("wire name %q shared by kinds %d and %d", name, uint8(prev), uint8(k))
		}
		seen[name] = k
	}
	// Out-of-range kinds degrade to plain rendering, never an empty string.
	if got := highlightKindName(highlight.TokenKind(255)); got != "plain" {
		t.Fatalf("unknown kind wire name = %q, want plain", got)
	}
}

// TestFileSpansShape asserts /api/file carries spans aligned with content:
// line indices in range, ordered byte offsets, known kind names.
func TestFileSpansShape(t *testing.T) {
	srv, root := testServer(t)
	cookies := loginCookies(t, srv)

	target := "/api/file?root=" + url.QueryEscape(root) + "&path=" + url.QueryEscape(root+"/main.go") + "&mode=full"
	rec := do(srv, "GET", target, nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("file = %d", rec.Code)
	}
	var decoded struct {
		Content string   `json:"content"`
		Spans   [][4]any `json:"spans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Spans) == 0 {
		t.Fatal("go preview has no spans")
	}
	lineCount := len(splitLines(decoded.Content))
	for _, sp := range decoded.Spans {
		li, ok1 := sp[0].(float64)
		start, ok2 := sp[1].(float64)
		end, ok3 := sp[2].(float64)
		kind, ok4 := sp[3].(string)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			t.Fatalf("span tuple has wrong shape: %v", sp)
		}
		if int(li) < 0 || int(li) >= lineCount {
			t.Fatalf("span line %v out of range (%d lines)", li, lineCount)
		}
		if start < 0 || end <= start {
			t.Fatalf("span offsets unordered: %v", sp)
		}
		if kind == "" || kind == "plain" {
			t.Fatalf("span kind %q should not be emitted", kind)
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
