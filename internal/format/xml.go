package format

import (
	"fmt"
	"html"
	"io"
	"strings"
)

type xmlRenderer struct{}

// Render writes a repository document: an optional instructions block, a
// structure tree, then each file's content wrapped in a CDATA section.
func (r *xmlRenderer) Render(doc *Document, w io.Writer) error {
	if _, err := fmt.Fprint(w, "<repository>\n"); err != nil {
		return err
	}

	if doc.Instructions != "" {
		fmt.Fprint(w, "  <instructions>\n")
		for _, line := range strings.Split(strings.TrimRight(doc.Instructions, "\n"), "\n") {
			fmt.Fprintf(w, "    %s\n", html.EscapeString(line))
		}
		fmt.Fprint(w, "  </instructions>\n")
	}

	if doc.DirectoryTree != "" {
		fmt.Fprint(w, "  <structure>\n")
		for _, line := range strings.Split(strings.TrimRight(doc.DirectoryTree, "\n"), "\n") {
			fmt.Fprintf(w, "    %s\n", html.EscapeString(line))
		}
		fmt.Fprint(w, "  </structure>\n")
	}

	if len(doc.Files) > 0 {
		fmt.Fprint(w, "  <files>\n")
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

			fmt.Fprintf(w, "    <file path=\"%s\" language=\"%s\" tokens=\"%d\" mode=\"%s\">\n",
				pathAttr, langAttr, f.Tokens, mode)
			fmt.Fprintf(w, "<![CDATA[%s]]>\n", cdataContent(f.Content))
			fmt.Fprint(w, "    </file>\n")
		}
		fmt.Fprint(w, "  </files>\n")
	}

	fmt.Fprint(w, "</repository>\n")
	return nil
}

// cdataContent splits any "]]>" sequence inside content so it cannot
// terminate the enclosing CDATA section early.
func cdataContent(content []byte) string {
	return strings.ReplaceAll(string(content), "]]>", "]]]]><![CDATA[>")
}
