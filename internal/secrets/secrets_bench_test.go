package secrets

import (
	"strings"
	"testing"
)

// benchCleanContent mimics a typical source file with no secrets: the keyword
// gate short-circuits most rules, but the full rule set still runs per file.
var benchCleanContent = []byte(strings.Repeat(`func Handler(w http.ResponseWriter, r *http.Request) {
	// handle the request payload and return a JSON response
	fmt.Fprintf(w, "%s", r.URL.Path)
}
`, 40))

// benchSecretContent is the same file plus one AWS key so a real match path
// (detection + splice) is exercised.
var benchSecretContent = append(
	[]byte("const aws = \"AKIAIOSFODNN7EXAMPLE\"\n"),
	benchCleanContent...,
)

// BenchmarkRedactContentClean measures the common case: a file with no
// secrets, where every rule is gated but none match.
func BenchmarkRedactContentClean(b *testing.B) {
	s := New()
	b.ReportAllocs()
	b.SetBytes(int64(len(benchCleanContent)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.RedactContent(benchCleanContent)
	}
}

// BenchmarkRedactContentSecret measures the detection path: at least one rule
// matches and the content is redacted in place.
func BenchmarkRedactContentSecret(b *testing.B) {
	s := New()
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSecretContent)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.RedactContent(benchSecretContent)
	}
}
