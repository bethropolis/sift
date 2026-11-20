package format

import (
	"encoding/xml"
	"strings"
	"testing"
)

type contextUpdateXML struct {
	XMLName xml.Name `xml:"context_update"`
	Type    string   `xml:"type,attr"`
	From    string   `xml:"from_commit,attr"`
	To      string   `xml:"to_commit,attr"`
	Body    string   `xml:",chardata"`
}

func TestRenderContextUpdateEscapesXML(t *testing.T) {
	patch := "--- a/x.xml\n+++ b/x.xml\n@@ -1 +1 @@\n-<a>]]></a>\n+<b>&amp;</b>\n"
	out := RenderContextUpdate(`ref"<&`, `HEAD>`, patch)

	var got contextUpdateXML
	if err := xml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, out)
	}
	if got.Type != "delta_patch" {
		t.Errorf("type = %q", got.Type)
	}
	if got.From != `ref"<&` {
		t.Errorf("from = %q, want %q", got.From, `ref"<&`)
	}
	if got.To != "HEAD>" {
		t.Errorf("to = %q", got.To)
	}
	if !strings.Contains(got.Body, patch) {
		t.Errorf("patch did not round-trip:\n--- body ---\n%s\n--- want ---\n%s", got.Body, patch)
	}
}

func TestRenderContextUpdatePlainPatch(t *testing.T) {
	patch := "diff --git a/a.go b/a.go\n"
	out := RenderContextUpdate("abc123", "def456", patch)
	if !strings.Contains(out, "<![CDATA[") || !strings.Contains(out, "]]>") {
		t.Fatalf("missing CDATA envelope:\n%s", out)
	}
	if strings.Contains(out, "]]]]><![CDATA[>") {
		t.Error("unnecessary CDATA split")
	}
}
