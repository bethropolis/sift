package format

import (
	"html"
	"io"
	"strings"
)

type xmlRenderer struct{}

// Render writes a repository document: an optional instructions block, a
// structure tree, then each file's content wrapped in a CDATA section.
func (r *xmlRenderer) Render(doc *Document, w io.Writer) error {
	if err := fprint(w, "<repository>\n"); err != nil {
		return err
	}

	if doc.Instructions != "" {
		if err := fprint(w, "  <instructions>\n"); err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimRight(doc.Instructions, "\n"), "\n") {
			if err := fprint(w, "    %s\n", html.EscapeString(line)); err != nil {
				return err
			}
		}
		if err := fprint(w, "  </instructions>\n"); err != nil {
			return err
		}
	}

	if doc.DirectoryTree != "" {
		if err := fprint(w, "  <structure>\n"); err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimRight(doc.DirectoryTree, "\n"), "\n") {
			if err := fprint(w, "    %s\n", html.EscapeString(line)); err != nil {
				return err
			}
		}
		if err := fprint(w, "  </structure>\n"); err != nil {
			return err
		}
	}

	if len(doc.Files) > 0 {
		if err := fprint(w, "  <files>\n"); err != nil {
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

			if err := fprint(w, "    <file path=\"%s\" language=\"%s\" tokens=\"%d\" mode=\"%s\">\n",
				pathAttr, langAttr, f.Tokens, mode); err != nil {
				return err
			}
			if err := fprint(w, "<![CDATA[%s]]>\n", cdataContent(f.Content)); err != nil {
				return err
			}
			if err := fprint(w, "    </file>\n"); err != nil {
				return err
			}
		}
		if err := fprint(w, "  </files>\n"); err != nil {
			return err
		}
	}

	return fprint(w, "</repository>\n")
}

// cdataContent splits any "]]>" sequence inside content so it cannot
// terminate the enclosing CDATA section early.
func cdataContent(content []byte) string {
	return strings.ReplaceAll(string(content), "]]>", "]]]]><![CDATA[>")
}
