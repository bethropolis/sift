package format

import (
	"io"

	"github.com/bethropolis/sift/internal/highlight"
)

type plainRenderer struct {
	useColors bool
	highlight highlight.Options
}

// Render writes each file's path followed by its content.
func (r *plainRenderer) Render(doc *Document, w io.Writer) error {
	if doc.Instructions != "" {
		if r.useColors {
			if err := fprint(w, "\033[1;36m# Instructions\033[0m\n"); err != nil {
				return err
			}
		} else if err := fprint(w, "# Instructions\n"); err != nil {
			return err
		}
		if err := fprint(w, "%s\n\n", doc.Instructions); err != nil {
			return err
		}
	}

	for _, f := range doc.Files {
		if r.useColors {
			if err := fprint(w, "\033[1;36m%s\033[0m\n", f.Path); err != nil {
				return err
			}
		} else if err := fprint(w, "%s\n", f.Path); err != nil {
			return err
		}
		if r.highlight.Enabled {
			// Highlighting must work on a string; this is the only path that
			// copies content during render.
			content := highlight.Render(f.Path, f.Content, r.highlight)
			if err := fprint(w, "%s\n\n", content); err != nil {
				return err
			}
			continue
		}
		// Without highlighting the bytes are written directly, avoiding a
		// second full copy of every file's content during render. The path was
		// already written above.
		if _, err := w.Write(f.Content); err != nil {
			return err
		}
		if err := fprint(w, "\n\n"); err != nil {
			return err
		}
	}
	return nil
}

type markdownRenderer struct{}

// Render writes the directory tree followed by a fenced block per file.
func (r *markdownRenderer) Render(doc *Document, w io.Writer) error {
	if doc.Instructions != "" {
		if err := fprint(w, "## Task / Instructions\n\n%s\n\n", doc.Instructions); err != nil {
			return err
		}
	}

	if doc.DirectoryTree != "" {
		if err := fprint(w, "```\n%s```\n\n", doc.DirectoryTree); err != nil {
			return err
		}
	}

	for _, f := range doc.Files {
		lang := f.Language
		if lang == "" {
			lang = languageForPath(f.Path)
		}
		fence := negotiateFence(f.Content)
		if err := fprint(w, "file: %s\n\n%s%s\n%s\n%s\n\n", f.Path, fence, lang, f.Content, fence); err != nil {
			return err
		}
	}
	return nil
}
