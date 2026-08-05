package lang

// Blank imports pull in every language driver so each registers its Language
// (and, in cgo builds, its tree-sitter SignatureSpec) during init.
import (
	_ "github.com/bethropolis/sift/internal/lang/go"
	_ "github.com/bethropolis/sift/internal/lang/javascript"
	_ "github.com/bethropolis/sift/internal/lang/other"
	_ "github.com/bethropolis/sift/internal/lang/python"
	_ "github.com/bethropolis/sift/internal/lang/rust"
	_ "github.com/bethropolis/sift/internal/lang/typescript"
)
