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

// jsonDoc is the JSON document used when instructions are present.
type jsonDoc struct {
	Instructions string          `json:"instructions,omitempty"`
	Files        []JSONFileEntry `json:"files"`
}

type jsonRenderer struct{}

// Render writes a JSON document. Without instructions the output is a plain
// array of file entries for backward compatibility; with instructions it is
// wrapped in an object that also carries the task directives.
func (r *jsonRenderer) Render(doc *Document, w io.Writer) error {
	entries := make([]JSONFileEntry, 0, len(doc.Files))
	for _, f := range doc.Files {
		entries = append(entries, JSONFileEntry{
			Path:    f.Path,
			Content: base64.StdEncoding.EncodeToString(f.Content),
			Tokens:  f.Tokens,
		})
	}

	if doc.Instructions == "" {
		if len(entries) == 0 {
			fmt.Fprint(w, "[]\n")
			return nil
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("format: marshal json: %w", err)
		}
		fmt.Fprintf(w, "%s\n", data)
		return nil
	}

	data, err := json.MarshalIndent(jsonDoc{Instructions: doc.Instructions, Files: entries}, "", "  ")
	if err != nil {
		return fmt.Errorf("format: marshal json: %w", err)
	}
	fmt.Fprintf(w, "%s\n", data)
	return nil
}
