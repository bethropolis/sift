package format

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

type cdataRepo struct {
	Files []struct {
		Path    string `xml:"path,attr"`
		Content string `xml:",chardata"`
	} `xml:"files>file"`
}

func renderXMLDoc(t *testing.T, content []byte) (string, []Section) {
	t.Helper()
	var sections []Section
	renderer, err := NewRendererWithOptions(StyleXML, RenderOptions{Sections: &sections})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	doc := &Document{Files: []FileEntry{{Path: "x.txt", Content: content, Tokens: 1}}}
	if err := renderer.Render(doc, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String(), sections
}

func TestXMLCDATAEdgeCases(t *testing.T) {
	cases := map[string][]byte{
		"normal":   []byte("plain content\n"),
		"single":   []byte("a ]]>\n"),
		"adjacent": []byte("x ]]>]]> y"),
		"start":    []byte("]]>rest"),
		"end":      []byte("tail]]>"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			out, _ := renderXMLDoc(t, content)
			var parsed cdataRepo
			if err := xml.Unmarshal([]byte(out), &parsed); err != nil {
				t.Fatalf("invalid XML: %v\n%s", err, out)
			}
			if len(parsed.Files) != 1 {
				t.Fatalf("files = %d", len(parsed.Files))
			}
			got := strings.TrimSuffix(strings.TrimPrefix(parsed.Files[0].Content, "\n"), "\n    ")
			if got != string(content) {
				t.Fatalf("content round-trip = %q, want %q", got, content)
			}
		})
	}
}

func TestXMLSectionsOffsets(t *testing.T) {
	content := []byte("line one\nline ]]> two\n")
	out, sections := renderXMLDoc(t, content)
	if len(sections) != 1 {
		t.Fatalf("sections = %d", len(sections))
	}
	s := sections[0]
	if s.Start < 0 || s.End > len(out) || s.Start >= s.End {
		t.Fatalf("bad section bounds %+v for %d bytes", s, len(out))
	}
	block := out[s.Start:s.End]
	if !strings.Contains(block, `<file path="x.txt"`) || !strings.Contains(block, "</file>") {
		t.Fatalf("section does not cover the file block:\n%s", block)
	}
	if !strings.Contains(block, "line ]]> two") && !strings.Contains(block, "]]]]><![CDATA[>") {
		t.Fatalf("section missing content:\n%s", block)
	}
}

func BenchmarkXMLRenderPlainContent(b *testing.B) {
	renderer, err := NewRenderer(StyleXML, false)
	if err != nil {
		b.Fatal(err)
	}
	content := bytes.Repeat([]byte("func example() { return nil }\n"), 200)
	doc := &Document{Files: []FileEntry{{Path: "big.go", Content: content, Tokens: 100}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := renderer.Render(doc, ioDiscard{}); err != nil {
			b.Fatal(err)
		}
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
