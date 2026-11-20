package app

import (
	"reflect"
	"testing"
)

func TestArchitectureEdges(t *testing.T) {
	graph := map[string][]string{
		"internal/app/render.go":   {"internal/format", "internal/app/graph.go", "internal/tokenize/tokenize.go"},
		"internal/app/graph.go":    {"internal/format/format.go", "internal/app/render.go"},
		"internal/format/xml.go":   {"internal/format/format.go"},
		"internal/scan/reader.go":  {},
		"internal/tokenize/tok.go": {},
	}
	got := ArchitectureEdges(graph)
	want := []string{
		"internal/app -> internal/format",
		"internal/app -> internal/tokenize",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ArchitectureEdges = %v, want %v", got, want)
	}
}

func TestArchitectureEdgesDropsSelfAndUnknown(t *testing.T) {
	graph := map[string][]string{
		"pkg/a.go": {"pkg/a.go", "unknown/target.go"},
	}
	if got := ArchitectureEdges(graph); len(got) != 0 {
		t.Fatalf("expected no edges, got %v", got)
	}
}

func TestArchitectureEdgesIncludesLeafPackageTargets(t *testing.T) {
	graph := map[string][]string{
		"main.go": {"helper"},
	}
	got := ArchitectureEdges(graph)
	want := []string{". -> helper"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ArchitectureEdges = %v, want %v", got, want)
	}
}
