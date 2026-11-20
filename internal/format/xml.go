package format

import (
	"bytes"
	"html"
	"io"
	"strings"
)

type xmlRenderer struct {
	sections *[]Section
}

// Render writes a repository document: an optional instructions block, a
// structure tree, then each file's content wrapped in a CDATA section.
func (r *xmlRenderer) Render(doc *Document, w io.Writer) error {
	out, cw := sectionOut(w, r.sections)
	if err := fprint(out, "<repository>\n"); err != nil {
		return err
	}

	if doc.Instructions != "" {
		if err := fprint(out, "  <instructions>\n"); err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimRight(doc.Instructions, "\n"), "\n") {
			if err := fprint(out, "    %s\n", html.EscapeString(line)); err != nil {
				return err
			}
		}
		if err := fprint(out, "  </instructions>\n"); err != nil {
			return err
		}
	}

	if doc.DirectoryTree != "" {
		if err := fprint(out, "  <structure>\n"); err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimRight(doc.DirectoryTree, "\n"), "\n") {
			if err := fprint(out, "    %s\n", html.EscapeString(line)); err != nil {
				return err
			}
		}
		if err := fprint(out, "  </structure>\n"); err != nil {
			return err
		}
	}

	if len(doc.Files) > 0 {
		if err := fprint(out, "  <files>\n"); err != nil {
			return err
		}
		for _, f := range doc.Files {
			lang := f.Language
			if lang == "" {
				lang = languageForPath(f.Path)
			}
			mode := "full"
			if f.IsCompressed {
				mode = "signatures"
			}

			pathAttr := html.EscapeString(f.Path)
			langAttr := html.EscapeString(lang)

			start := 0
			if cw != nil {
				start = cw.n
			}
			if err := fprint(out, "    <file path=\"%s\" language=\"%s\" tokens=\"%d\" mode=\"%s\">\n",
				pathAttr, langAttr, f.Tokens, mode); err != nil {
				return err
			}
			if err := fprint(out, "<![CDATA["); err != nil {
				return err
			}
			if err := writeCDATA(out, f.Content); err != nil {
				return err
			}
			if err := fprint(out, "]]>\n"); err != nil {
				return err
			}
			if err := fprint(out, "    </file>\n"); err != nil {
				return err
			}
			recordSection(r.sections, cw, f.Path, f.Tokens, start)
		}
		if err := fprint(out, "  </files>\n"); err != nil {
			return err
		}
	}

	return fprint(out, "</repository>\n")
}

// writeCDATA writes content directly, splitting any "]]>" sequence so it
// cannot terminate the enclosing CDATA section early. Bytes are written as
// slices of the caller's buffer; no string conversion or full-content copy is
// made on the common path.
func writeCDATA(w io.Writer, content []byte) error {
	marker := []byte("]]>")
	for {
		idx := bytes.Index(content, marker)
		if idx < 0 {
			_, err := w.Write(content)
			return err
		}
		if _, err := w.Write(content[:idx]); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "]]]]><![CDATA[>"); err != nil {
			return err
		}
		content = content[idx+len(marker):]
	}
}
