package summary

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/sift/internal/walker"
)

func TestPrintDumpSummary_Standard(t *testing.T) {
	var buf bytes.Buffer
	stats := DumpStats{
		FileCount:         42,
		FullCount:         30,
		SigCount:          12,
		SkippedCount:      8,
		TotalTokens:       45120,
		Budget:            150000,
		Duration:          35 * time.Millisecond,
		OutputFile:        "codebase.xml",
		OutputStyle:       "xml",
		IsClipboard:       false,
		SecretCount:       0,
		FallbackMode:      "signatures",
		FallbackThreshold: 0.9,
		FallbackApplied:   true,
	}

	PrintDumpSummary(&buf, stats, false, false)
	out := buf.String()

	if !strings.Contains(out, "Dump complete in 35ms") {
		t.Errorf("expected completion message, got:\n%s", out)
	}
	if !strings.Contains(out, "codebase.xml (xml)") {
		t.Errorf("expected output file, got:\n%s", out)
	}
	if !strings.Contains(out, "42 included (30 full, 12 signatures) · 8 skipped") {
		t.Errorf("expected files breakdown, got:\n%s", out)
	}
	if !strings.Contains(out, "45,120 / 150,000 (30.1%)") {
		t.Errorf("expected token breakdown, got:\n%s", out)
	}
	if !strings.Contains(out, "None detected") {
		t.Errorf("expected secret status, got:\n%s", out)
	}
	if !strings.Contains(out, "signatures (threshold: 90%) [active]") {
		t.Errorf("expected fallback info, got:\n%s", out)
	}
}

func TestPrintDumpSummary_ClipboardAndSecrets(t *testing.T) {
	var buf bytes.Buffer

	stats := DumpStats{
		FileCount:    10,
		FullCount:    10,
		SigCount:     0,
		SkippedCount: 0,
		TotalTokens:  5000,
		Budget:       0, // unlimited
		Duration:     12 * time.Millisecond,
		IsClipboard:  true,
		OutputStyle:  "markdown",
		SecretCount:  3,
	}

	PrintDumpSummary(&buf, stats, false, false)
	out := buf.String()

	if !strings.Contains(out, "Clipboard (markdown)") {
		t.Errorf("expected clipboard output, got:\n%s", out)
	}
	if !strings.Contains(out, "5,000 (unlimited)") {
		t.Errorf("expected unlimited budget tokens, got:\n%s", out)
	}
	if !strings.Contains(out, "3 redacted") {
		t.Errorf("expected redacted secrets, got:\n%s", out)
	}
}

func TestPrintDumpSummary_Quiet(t *testing.T) {
	var buf bytes.Buffer

	stats := DumpStats{
		FileCount:   5,
		TotalTokens: 1000,
		Duration:    10 * time.Millisecond,
	}

	PrintDumpSummary(&buf, stats, false, true)
	if buf.Len() != 0 {
		t.Errorf("expected no output when quiet=true, got: %q", buf.String())
	}
}

func TestDisplaySkippedItems(t *testing.T) {
	var buf bytes.Buffer

	items := []walker.SkippedItem{
		{Path: "node_modules", IsDir: true, Reason: "ignored"},
		{Path: "bundle.min.js", IsDir: false, Reason: "minified"},
	}

	DisplaySkippedItems(nil, items, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "Skipped Items (2)") {
		t.Errorf("expected skipped items header, got:\n%s", out)
	}
	if !strings.Contains(out, "DIR   node_modules") {
		t.Errorf("expected dir skipped, got:\n%s", out)
	}
	if !strings.Contains(out, "FILE  bundle.min.js") {
		t.Errorf("expected file skipped, got:\n%s", out)
	}
}
