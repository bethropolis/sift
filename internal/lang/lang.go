// Package lang is the facade external consumers import. It centralizes every
// language-specific decision in Sift: file-path role classification for
// ranking, smart-filter rules, and (in cgo builds) tree-sitter signature
// extraction. Concrete types live in the leaf types package, global tables in
// registry, and the signature engine in the cgo-only signature subpackage;
// language drivers register themselves via init() and are pulled in by
// imports.go.
package lang

import (
	"github.com/bethropolis/sift/internal/lang/registry"
	"github.com/bethropolis/sift/internal/lang/types"
)

// ID uniquely identifies a supported language.
type ID = types.ID

// Role describes the likely architecture purpose of a file.
type Role = types.Role

// Classification is a soft selection signal for ranking and budgeting.
type Classification = types.Classification

// Language is the contract every driver implements.
type Language = types.Language

// SignatureResolver is an optional capability of a Language that maps a path
// to the signature registry ID for it.
type SignatureResolver = types.SignatureResolver

// Language IDs.
const (
	Go         ID = types.Go
	JavaScript ID = types.JavaScript
	TypeScript ID = types.TypeScript
	TSX        ID = types.TSX
	Python     ID = types.Python
	Rust       ID = types.Rust
	Java       ID = types.Java
	Kotlin     ID = types.Kotlin
	CSharp     ID = types.CSharp
	Cpp        ID = types.Cpp
	Ruby       ID = types.Ruby
	PHP        ID = types.PHP
	Swift      ID = types.Swift
	Dart       ID = types.Dart
	Zig        ID = types.Zig
	Other      ID = types.Other
)

// File roles.
const (
	RoleUnknown    Role = types.RoleUnknown
	RoleEntrypoint Role = types.RoleEntrypoint
	RoleAPI        Role = types.RoleAPI
	RoleImpl       Role = types.RoleImpl
	RoleTest       Role = types.RoleTest
	RoleFixture    Role = types.RoleFixture
	RoleMock       Role = types.RoleMock
	RoleConfig     Role = types.RoleConfig
	RoleSchema     Role = types.RoleSchema
	RoleDocs       Role = types.RoleDocs
	RoleGenerated  Role = types.RoleGenerated
	RoleExample    Role = types.RoleExample
	RoleVendor     Role = types.RoleVendor
)

// DeclMap builds a lookup set of AST node types for a SignatureSpec.
func DeclMap(tags ...string) map[string]bool { return types.DeclMap(tags...) }

// Register adds a language to the global registry.
func Register(l Language) { registry.Register(l) }

// ByID returns the language registered under id.
func ByID(id ID) (Language, bool) { return registry.ByID(id) }

// ForPath resolves the language for a file by extension.
func ForPath(path string) (Language, bool) { return registry.ForPath(path) }

// Classify applies shared conventions and then the language driver rules.
func Classify(path string) Classification { return registry.Classify(path) }

// Imports returns the import targets referenced by a file, resolved to
// repo-relative path prefixes. Paths whose language has no ImportScanner
// contribute no fan-in.
func Imports(path, moduleRoot string, content []byte) []string {
	return registry.Imports(path, moduleRoot, content)
}

// HasImportScanner reports whether the language for path extracts imports.
func HasImportScanner(path string) bool { return registry.HasImportScanner(path) }

// ImportExtensions lists the sorted extensions of import-scanning languages.
func ImportExtensions() []string { return registry.ImportExtensions() }

// ResolveImports returns the import targets referenced by a file as
// confirmed, existing repo-relative file paths. Drivers implementing
// ImportResolver commit to real files; all other paths fall back to generic
// prefix expansion. exists reports whether a candidate path was collected.
func ResolveImports(path, moduleRoot string, content []byte, exists func(string) bool) []string {
	return registry.ResolveImports(path, moduleRoot, content, exists)
}

// ShouldSkipSmart reports whether any registered language's smart rules skip
// the file.
func ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	return registry.ShouldSkipSmart(path, filename, content)
}
