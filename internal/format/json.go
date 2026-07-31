package format

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// JSONFileEntry is the wire format for a single file in JSON output.
type JSONFileEntry struct {
	Path    string `json:"path"`
	Content string `json:"content"` // Base64 encoded content
	Tokens  int    `json:"tokens,omitempty"`
}

type jsonRenderer struct{}

// Render writes a JSON array of file entries, or an empty array if there are none.
func (r *jsonRenderer) Render(doc *Document, w io.Writer) error {
	if len(doc.Files) == 0 {
		fmt.Fprint(w, "[]\n")
		return nil
	}

	entries := make([]JSONFileEntry, 0, len(doc.Files))
	for _, f := range doc.Files {
		entries = append(entries, JSONFileEntry{
			Path:    f.Path,
			Content: base64.StdEncoding.EncodeToString(f.Content),
			Tokens:  f.Tokens,
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("format: marshal json: %w", err)
	}
	fmt.Fprintf(w, "%s\n", data)
	return nil
}
