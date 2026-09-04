package langother

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bethropolis/sift/internal/lang/registry"
	"github.com/bethropolis/sift/internal/lang/types"
)

type otherDriver struct{}

func init() { registry.Register(otherDriver{}) }

func (otherDriver) ID() types.ID { return types.Other }
func (otherDriver) Name() string { return "Other" }
func (otherDriver) Extensions() []string {
	return []string{".java", ".kt", ".kts", ".rb", ".php", ".cs", ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".swift"}
}
func (otherDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	const reason = "Matched smart language filter"
	if filename == "gradle-wrapper.jar" || filename == "gradlew.bat" {
		return true, reason
	}
	if strings.HasSuffix(filename, ".class") || strings.HasSuffix(filename, ".jar") || strings.HasSuffix(filename, ".aar") {
		return true, reason
	}
	if filename == "composer.lock" || filename == "gemfile.lock" {
		return true, reason
	}
	return false, ""
}

func (d otherDriver) Classify(path, filename string) types.Classification {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".java":
		return jvmFile(path, filename, "Java source", ".java")
	case ".kt", ".kts":
		return jvmFile(path, filename, "Kotlin source", ".kt")
	case ".rb":
		return conventionByName(filename, "Ruby source")
	case ".php":
		return conventionByName(filename, "PHP source")
	case ".cs":
		return csharpFile(path, filename)
	case ".h", ".hpp":
		if strings.Contains(path, "/include/") || strings.HasPrefix(path, "include/") {
			return types.Classification{Role: types.RoleAPI, Adjustment: 0.14, Confidence: 0.90, Reason: "C/C++ public header"}
		}
		return types.Classification{Role: types.RoleAPI, Adjustment: 0.08, Confidence: 0.80, Reason: "C/C++ header"}
	case ".c":
		return conventionByName(filename, "C source")
	case ".cc", ".cpp", ".cxx":
		return conventionByName(filename, "C++ source")
	case ".swift":
		return conventionByName(filename, "Swift source")
	}
	return types.Classification{Role: types.RoleUnknown}
}

var (
	cIncludeRegex    = regexp.MustCompile(`(?m)^\s*#\s*include\s*["<]([^">]+)[">]`)
	rubyRequireRegex = regexp.MustCompile(`(?m)^\s*require_relative\s*['"]([^'"]+)['"]`)
	phpRequireRegex  = regexp.MustCompile(`(?m)\b(?:require|require_once|include|include_once)\s*\(?\s*['"]([^'"]+)['"]`)
)

// Imports returns local import targets for files handled by the otherDriver
// (C/C++ local headers, Ruby require_relative, PHP relative includes).
func (otherDriver) Imports(path, _ string, content []byte) []string {
	dir := filepath.ToSlash(filepath.Dir(path))
	ext := strings.ToLower(filepath.Ext(path))
	var targets []string

	switch ext {
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp":
		for _, m := range cIncludeRegex.FindAllSubmatch(content, -1) {
			inc := string(m[1])
			// Resolve relative to current file dir, and also add raw inc path
			// (for include/<inc> style references).
			target := filepath.ToSlash(filepath.Clean(filepath.Join(dir, inc)))
			targets = append(targets, target)
			if inc != target {
				targets = append(targets, filepath.ToSlash(inc))
			}
		}
	case ".rb":
		for _, m := range rubyRequireRegex.FindAllSubmatch(content, -1) {
			req := string(m[1])
			target := filepath.ToSlash(filepath.Clean(filepath.Join(dir, req)))
			targets = append(targets, target)
		}
	case ".php":
		for _, m := range phpRequireRegex.FindAllSubmatch(content, -1) {
			inc := string(m[1])
			if strings.HasPrefix(inc, "./") || strings.HasPrefix(inc, "../") {
				target := filepath.ToSlash(filepath.Clean(filepath.Join(dir, inc)))
				targets = append(targets, target)
			}
		}
	}
	return targets
}

// SignatureLanguage maps the secondary extensions this driver owns to the
// signature registry ID that carries their grammar and declaration rules.
func (d otherDriver) SignatureLanguage(path string) types.ID {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".php":
		return types.PHP
	case ".java":
		return types.Java
	case ".kt", ".kts":
		return types.Kotlin
	case ".cs":
		return types.CSharp
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp":
		return types.Cpp
	case ".rb":
		return types.Ruby
	case ".swift":
		return types.Swift
	}
	return types.Other
}

func csharpFile(path, base string) types.Classification {
	if strings.HasSuffix(base, "test.cs") || strings.HasSuffix(base, "tests.cs") {
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.90, Reason: "C# test file"}
	}
	if base == "program.cs" || base == "startup.cs" {
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "C# application entrypoint"}
	}
	return conventionByName(base, "C# source")
}

func conventionByName(base, label string) types.Classification {
	if base == "main.java" || base == "main.kt" || base == "main.rb" || base == "main.php" || base == "main.cs" || base == "main.c" || base == "main.cpp" || base == "main.swift" {
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.80, Reason: label + " entrypoint"}
	}
	return types.Classification{Role: types.RoleImpl, Confidence: 0.45, Reason: label}
}

func jvmFile(path, base, label, ext string) types.Classification {
	if strings.Contains(path, "/src/test/") || strings.HasSuffix(base, "test"+ext) || strings.HasSuffix(base, "tests"+ext) || strings.HasSuffix(base, "it"+ext) {
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.90, Reason: label + " test file"}
	}
	if base == "application"+ext || base == "main"+ext {
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: label + " entrypoint"}
	}
	if strings.HasSuffix(base, "controller"+ext) || strings.HasSuffix(base, "service"+ext) || strings.HasSuffix(base, "api"+ext) {
		return types.Classification{Role: types.RoleAPI, Adjustment: 0.10, Confidence: 0.75, Reason: label + " API layer"}
	}
	return conventionByName(base, label)
}
