// Package types defines the core language contracts shared by every driver and
// the registry. It is a leaf package: drivers import it, and nothing inside it
// imports back, which keeps the driver/facade graph cycle-free.
package types

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
	// Retention is the default importance of the file's role: how strongly
	// the utility optimizer should keep it under a token budget. Zero means
	// the role carries no retention guarantee. Drivers and the shared role
	// table may set it; the config layer can override per role.
	Retention float64
}

// Language is the contract every driver implements. Signature extraction
// (Grammar + SignatureSpec) is provided separately through the cgo-only
// signature package so this common interface builds without cgo.
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

// ImportScanner is an optional capability of a Language that returns the
// import paths referenced by a file's content. It powers reverse import
// fan-in for centrality scoring. Import syntax is language knowledge, so the
// parsing and module-root stripping rules live in each driver rather than in
// the ranker.
type ImportScanner interface {
	// Imports returns the import targets referenced by content as
	// repo-relative path prefixes (e.g. "internal/app" for a Go import of
	// "example.com/mod/internal/app"). moduleRoot is the repo's module path
	// (e.g. the go.mod module line); drivers use it to strip the module
	// prefix from absolute imports. Unresolvable or external references may
	// be omitted.
	Imports(path, moduleRoot string, content []byte) []string
}

// DeclMap builds a lookup set of AST node types for a SignatureSpec.
func DeclMap(tags ...string) map[string]bool {
	m := make(map[string]bool, len(tags))
	for _, t := range tags {
		m[t] = true
	}
	return m
}

// DefaultRetention returns the baseline retention priority for a role: how
// strongly the utility optimizer should keep files of that role under a token
// budget. Entrypoints and docs carry the most context for an LLM, so they get
// the highest guarantees; derivations and noise get none.
func DefaultRetention(role Role) float64 {
	switch role {
	case RoleEntrypoint:
		return 0.30
	case RoleDocs:
		return 0.22
	case RoleConfig:
		return 0.18
	case RoleAPI, RoleSchema:
		return 0.12
	case RoleImpl:
		return 0.05
	default:
		return 0.0
	}
}
