package compress

import (
	"strings"
	"testing"
)

func TestSummarizeSQLKeepsDDLDropsBulkRows(t *testing.T) {
	src := `-- schema
CREATE TABLE users (
  id integer PRIMARY KEY,
  name text
);

INSERT INTO users (id, name) VALUES (1, 'a');
INSERT INTO users (id, name) VALUES (2, 'b');
CREATE INDEX users_name ON users(name);
COPY users FROM stdin;
`
	got, ok := SummarizePath([]byte(src), "db/schema.sql")
	if !ok {
		t.Fatal("expected SQL summary")
	}
	if !strings.Contains(got, "CREATE TABLE users") || !strings.Contains(got, "CREATE INDEX users_name") {
		t.Fatalf("DDL missing:\n%s", got)
	}
	if strings.Contains(got, "VALUES (1, 'a')") {
		t.Fatalf("bulk insert leaked:\n%s", got)
	}
	if !strings.Contains(got, "3 bulk INSERT/COPY statements omitted") {
		t.Fatalf("omission count missing:\n%s", got)
	}
}

func TestSummarizeJSONShape(t *testing.T) {
	src := `{"openapi":"3.0.0","paths":{"/pets":{"get":{"summary":"List pets","responses":{"200":{"description":"ok"}}}},"/owners":{"get":{"summary":"List owners"}}},"tags":[{"name":"pets","description":"` + strings.Repeat("x", 200) + `"}]}`
	got, ok := SummarizePath([]byte(src), "api/openapi.json")
	if !ok {
		t.Fatal("expected JSON summary")
	}
	for _, want := range []string{`"openapi"`, `"paths"`, `"get"`, `/* 1 items */`, `"name"`} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, strings.Repeat("x", 100)) {
		t.Errorf("long string not truncated:\n%s", got)
	}
	if _, ok := SummarizePath([]byte("{broken"), "x.json"); ok {
		t.Error("invalid JSON should not summarize")
	}
}

func TestSummarizeYAMLKeysAndRuns(t *testing.T) {
	var b strings.Builder
	b.WriteString("openapi: 3.0.0\ninfo:\n  title: Demo\n  version: 1.0.0\npaths:\n  /pets:\n    get:\n      summary: List\nservers:\n")
	for i := 0; i < 10; i++ {
		b.WriteString("  - url: https://example.com\n")
	}
	got, ok := SummarizePath([]byte(b.String()), "spec.yaml")
	if !ok {
		t.Fatal("expected YAML summary")
	}
	if !strings.Contains(got, "openapi:") || !strings.Contains(got, "  title:") || !strings.Contains(got, "summary:") {
		t.Fatalf("keys missing:\n%s", got)
	}
	if !strings.Contains(got, "list items omitted") {
		t.Fatalf("list run not collapsed:\n%s", got)
	}
	if strings.Contains(got, "https://example.com") {
		t.Fatalf("scalar payload leaked:\n%s", got)
	}
}
