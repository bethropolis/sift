package format

import (
	"io"

	"github.com/bethropolis/sift/internal/highlight"
)

type plainRenderer struct {
	useColors bool
	highlight highlight.Options
	sections  *[]Section
}

// Render writes each file's path followed by its content.
func (r *plainRenderer) Render(doc *Document, w io.Writer) error {
	out, cw := sectionOut(w, r.sections)
	if doc.Instructions != "" {
		if r.useColors {
			if err := fprint(out, "\033[1;36m# Instructions\033[0m\n"); err != nil {
				return err
			}
		} else if err := fprint(out, "# Instructions\n"); err != nil {
			return err
		}
		if err := fprint(out, "%s\n\n", doc.Instructions); err != nil {
			return err
		}
	}

	for _, f := range doc.Files {
		start := 0
		if cw != nil {
			start = cw.n
		}
		if r.useColors {
			if err := fprint(out, "\033[1;36m%s\033[0m\n", f.Path); err != nil {
				return err
			}
		} else if err := fprint(out, "%s\n", f.Path); err != nil {
			return err
		}
		if r.highlight.Enabled {
			// Highlighting must work on a string; this is the only path that
			// copies content during render.
			content := highlight.Render(f.Path, f.Content, r.highlight)
			if err := fprint(out, "%s\n\n", content); err != nil {
				return err
			}
			recordSection(r.sections, cw, f.Path, f.Tokens, start)
			continue
		}
		// Without highlighting the bytes are written directly, avoiding a
		// second full copy of every file's content during render. The path was
		// already written above.
		if _, err := out.Write(f.Content); err != nil {
			return err
		}
		if err := fprint(out, "\n\n"); err != nil {
			return err
		}
		recordSection(r.sections, cw, f.Path, f.Tokens, start)
	}
	return nil
}

type markdownRenderer struct {
	sections *[]Section
}

// Render writes the directory tree followed by a fenced block per file.
func (r *markdownRenderer) Render(doc *Document, w io.Writer) error {
	out, cw := sectionOut(w, r.sections)
	if doc.Instructions != "" {
		if err := fprint(out, "## Task / Instructions\n\n%s\n\n", doc.Instructions); err != nil {
			return err
		}
	}

	if doc.DirectoryTree != "" {
		if err := fprint(out, "```\n%s```\n\n", doc.DirectoryTree); err != nil {
			return err
		}
	}

	for _, f := range doc.Files {
		lang := f.Language
		if lang == "" {
			lang = languageForPath(f.Path)
		}
		fence := negotiateFence(f.Content)
		start := 0
		if cw != nil {
			start = cw.n
		}
		if err := fprint(out, "file: %s\n\n%s%s\n%s\n%s\n\n", f.Path, fence, lang, f.Content, fence); err != nil {
			return err
		}
		recordSection(r.sections, cw, f.Path, f.Tokens, start)
	}
	return nil
}
