// Package lang centralizes every language-specific decision in Sift: file-path
// role classification for ranking, smart-filter rules, and (in cgo builds)
// tree-sitter signature extraction. Language drivers live under
// internal/lang/<id>/ and register a Language in their init.
package lang

// ID uniquely identifies a supported language.
type ID string

const (
	Go         ID = "go"
	JavaScript ID = "javascript"
	TypeScript ID = "typescript"
	TSX        ID = "tsx"
	Python     ID = "python"
	Rust       ID = "rust"
	Java       ID = "java"
	Kotlin     ID = "kotlin"
	CSharp     ID = "csharp"
	Cpp        ID = "cpp"
	Ruby       ID = "ruby"
	PHP        ID = "php"
	Swift      ID = "swift"
	Other      ID = "other"
)

// Role describes the likely architecture purpose of a file.
type Role string

const (
	RoleUnknown    Role = "unknown"
	RoleEntrypoint Role = "entrypoint"
	RoleAPI        Role = "api"
	RoleImpl       Role = "implementation"
	RoleTest       Role = "test"
	RoleFixture    Role = "fixture"
	RoleMock       Role = "mock"
	RoleConfig     Role = "config"
	RoleSchema     Role = "schema"
	RoleDocs       Role = "docs"
	RoleGenerated  Role = "generated"
	RoleExample    Role = "example"
	RoleVendor     Role = "vendor"
)

// Classification is a soft selection signal for ranking and budgeting.
type Classification struct {
	Role       Role
	Adjustment float64
	Confidence float64
	Reason     string
}

// DeclMap builds a lookup set of AST node types for a SignatureSpec.
func DeclMap(types ...string) map[string]bool {
	m := make(map[string]bool, len(types))
	for _, t := range types {
		m[t] = true
	}
	return m
}

// Language is the contract every driver implements. Signature extraction
// (Grammar + SignatureSpec) is provided separately through a cgo-only registry
// so this common interface builds without cgo.
type Language interface {
	ID() ID
	Name() string
	Extensions() []string
	// Classify returns the architectural role of a normalized, lowercase,
	// slash-separated path and its lowercase basename.
	Classify(path, filename string) Classification
	// ShouldSkipSmart reports whether the path matches a smart-filter rule.
	ShouldSkipSmart(path, filename string, content []byte) (bool, string)
}

// SignatureResolver is an optional capability of a Language that maps a path to
// the signature registry ID for it. The catch-all "other" driver implements it
// so secondary languages resolve to their own grammar and declaration rules.
type SignatureResolver interface {
	SignatureLanguage(path string) ID
}
