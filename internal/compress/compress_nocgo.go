//go:build !cgo

// Package compress provides the portable fallback used by static builds.
// Tree-sitter's native grammars are unavailable without CGO, so callers keep
// the original source rather than producing an unsafe partial signature.
package compress

import (
	"path/filepath"
)

// Language identifies a supported grammar.
type Language int

const (
	Go Language = iota
	Rust
	JavaScript
	TypeScript
	TSX
	Python
	PHP
	Java
	Kotlin
	CSharp
	Cpp
	Ruby
	Swift
)

func (l Language) String() string {
	return [...]string{
		"go", "rust", "javascript", "typescript", "tsx", "python", "php",
		"java", "kotlin", "csharp", "cpp", "ruby", "swift",
	}[l]
}

var extToLang = map[string]Language{
	".go": Go, ".rs": Rust, ".js": JavaScript, ".jsx": JavaScript,
	".mjs": JavaScript, ".cjs": JavaScript, ".ts": TypeScript, ".tsx": TSX,
	".py": Python, ".pyi": Python, ".php": PHP, ".java": Java,
	".kt": Kotlin, ".kts": Kotlin, ".cs": CSharp, ".c": Cpp, ".h": Cpp,
	".cc": Cpp, ".cpp": Cpp, ".cxx": Cpp, ".rb": Ruby, ".swift": Swift,
}

// Compressor is a no-CGO compressor. It intentionally does not claim that a
// raw source file was compressed; scan then retains the full representation.
type Compressor struct{}

func New() *Compressor { return &Compressor{} }

func (c *Compressor) LanguageForPath(path string) (Language, bool) {
	lang, ok := extToLang[filepath.Ext(path)]
	return lang, ok
}

func (c *Compressor) Compress(_ []byte, _ Language) (string, bool) {
	return "", false
}
