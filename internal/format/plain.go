package format

import (
	"fmt"
	"io"
)

type plainRenderer struct {
	useColors bool
}

// Render writes each file's path followed by its content.
func (r *plainRenderer) Render(doc *Document, w io.Writer) error {
	for _, f := range doc.Files {
		if r.useColors {
			fmt.Fprintf(w, "\033[1;36m%s\033[0m\n", f.Path)
		} else {
			fmt.Fprintf(w, "%s\n", f.Path)
		}
		fmt.Fprintf(w, "%s\n\n", f.Content)
	}
	return nil
}

type markdownRenderer struct{}

// Render writes the directory tree followed by a fenced block per file.
func (r *markdownRenderer) Render(doc *Document, w io.Writer) error {
	if doc.DirectoryTree != "" {
		fmt.Fprintf(w, "```\n%s```\n\n", doc.DirectoryTree)
	}

	for _, f := range doc.Files {
		lang := f.Language
		if lang == "" {
			lang = languageForPath(f.Path)
		}
		fence := negotiateFence(f.Content)
		fmt.Fprintf(w, "file: %s\n\n%s%s\n%s\n%s\n\n", f.Path, fence, lang, f.Content, fence)
	}
	return nil
}
