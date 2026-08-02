package selection

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/walker"
)

// Report contains selection decisions and optional scan skips for display.
type Report struct {
	Result  Result
	Skipped []walker.SkippedItem
}

// Print writes a tree, JSON, or NDJSON report.
func (r Report) Print(w io.Writer, style string, includeSkipped bool) error {
	switch strings.ToLower(style) {
	case "json":
		return r.printJSON(w, includeSkipped)
	case "ndjson":
		return r.printNDJSON(w, includeSkipped)
	default:
		return r.printTree(w, includeSkipped)
	}
}

type jsonReport struct {
	Budget        int                  `json:"budget"`
	UsedTokens    int                  `json:"used_tokens"`
	SelectedFiles int                  `json:"selected_files"`
	Decisions     []Decision           `json:"decisions"`
	Skipped       []walker.SkippedItem `json:"skipped,omitempty"`
}

func (r Report) printJSON(w io.Writer, includeSkipped bool) error {
	skipped := r.Skipped
	if !includeSkipped {
		skipped = nil
	}
	data, err := json.MarshalIndent(jsonReport{
		Budget:        r.Result.Budget,
		UsedTokens:    r.Result.UsedTokens,
		SelectedFiles: len(r.Result.Selected),
		Decisions:     r.Result.Decisions,
		Skipped:       skipped,
	}, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func (r Report) printNDJSON(w io.Writer, includeSkipped bool) error {
	enc := json.NewEncoder(w)
	for _, decision := range r.Result.Decisions {
		if err := enc.Encode(decision); err != nil {
			return err
		}
	}
	if includeSkipped {
		for _, skipped := range r.Skipped {
			if err := enc.Encode(struct {
				Type string `json:"type"`
				walker.SkippedItem
			}{Type: "skipped", SkippedItem: skipped}); err != nil {
				return err
			}
		}
	}
	return enc.Encode(struct {
		Type          string `json:"type"`
		Budget        int    `json:"budget"`
		UsedTokens    int    `json:"used_tokens"`
		SelectedFiles int    `json:"selected_files"`
	}{"summary", r.Result.Budget, r.Result.UsedTokens, len(r.Result.Selected)})
}

type reportNode struct {
	children map[string]*reportNode
	decision *Decision
}

func (r Report) printTree(w io.Writer, includeSkipped bool) error {
	root := &reportNode{children: map[string]*reportNode{}}
	for i := range r.Result.Decisions {
		parts := strings.Split(r.Result.Decisions[i].Path, "/")
		node := root
		for _, part := range parts {
			if part == "" {
				continue
			}
			if node.children[part] == nil {
				node.children[part] = &reportNode{children: map[string]*reportNode{}}
			}
			node = node.children[part]
		}
		node.decision = &r.Result.Decisions[i]
	}

	if _, err := fmt.Fprintf(w, "Selection (budget: %d, used: %d)\n", r.Result.Budget, r.Result.UsedTokens); err != nil {
		return err
	}
	if err := printTreeNode(w, root, ""); err != nil {
		return err
	}
	if includeSkipped {
		for _, skipped := range r.Skipped {
			if _, err := fmt.Fprintf(w, "- %s  SKIP  %s\n", skipped.Path, skipped.Reason); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, "Selected: %d files\nTokens:   %d / %d\n", len(r.Result.Selected), r.Result.UsedTokens, r.Result.Budget)
	return err
}

func printTreeNode(w io.Writer, node *reportNode, prefix string) error {
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		child := node.children[name]
		last := i == len(names)-1
		connector := "├── "
		nextPrefix := prefix + "│   "
		if last {
			connector = "└── "
			nextPrefix = prefix + "    "
		}
		if child.decision != nil {
			d := child.decision
			if _, err := fmt.Fprintf(w, "%s%s%-32s %-4s %7d  %.2f  %s\n", prefix, connector, name, strings.ToUpper(string(d.Mode)), d.Tokens, d.Score, d.Reason); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "%s%s%s/\n", prefix, connector, name); err != nil {
			return err
		}
		if err := printTreeNode(w, child, nextPrefix); err != nil {
			return err
		}
	}
	return nil
}
