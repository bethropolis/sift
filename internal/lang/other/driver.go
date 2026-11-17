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
	return []string{".java", ".kt", ".kts", ".rb", ".php", ".cs", ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".swift", ".dart", ".zig"}
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
	if filename == "pubspec.lock" || strings.Contains(path, "/.dart_tool/") || strings.HasPrefix(path, ".dart_tool/") ||
		strings.Contains(path, "/.zig-cache/") || strings.HasPrefix(path, ".zig-cache/") || strings.Contains(path, "/zig-out/") || strings.HasPrefix(path, "zig-out/") {
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
		if filename == "settings.gradle.kts" || strings.HasSuffix(filename, ".gradle.kts") {
			return types.Classification{Role: types.RoleConfig, Adjustment: 0.12, Confidence: 0.95, Reason: "Gradle Kotlin DSL build configuration"}
		}
		return jvmFile(path, filename, "Kotlin source", ".kt")
	case ".rb":
		if filename == "routes.rb" {
			return types.Classification{Role: types.RoleAPI, Adjustment: 0.10, Confidence: 0.85, Reason: "Rails routes"}
		}
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
	case ".dart":
		if strings.HasSuffix(filename, "_test.dart") || strings.HasSuffix(filename, "_spec.dart") {
			return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.90, Reason: "Dart test file"}
		}
		if filename == "main.dart" {
			return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Dart/Flutter entrypoint"}
		}
		if strings.HasSuffix(filename, ".g.dart") || strings.HasSuffix(filename, ".freezed.dart") || strings.HasSuffix(filename, ".mocks.dart") {
			return types.Classification{Role: types.RoleGenerated, Adjustment: -0.35, Confidence: 0.95, Reason: "generated Dart source"}
		}
		return conventionByName(filename, "Dart/Flutter source")
	case ".zig":
		if strings.HasSuffix(filename, "_test.zig") {
			return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.90, Reason: "Zig test file"}
		}
		if filename == "main.zig" {
			return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Zig entrypoint"}
		}
		if filename == "root.zig" {
			return types.Classification{Role: types.RoleAPI, Adjustment: 0.10, Confidence: 0.80, Reason: "Zig package root"}
		}
		return conventionByName(filename, "Zig source")
	}
	return types.Classification{Role: types.RoleUnknown}
}

var (
	cIncludeRegex    = regexp.MustCompile(`(?m)^\s*#\s*include\s*["<]([^">]+)[">]`)
	rubyRequireRegex = regexp.MustCompile(`(?m)^\s*require_relative\s*['"]([^'"]+)['"]`)
	phpRequireRegex  = regexp.MustCompile(`(?m)\b(?:require|require_once|include|include_once)\s*\(?\s*['"]([^'"]+)['"]`)
	javaPackageRegex = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)`)
	javaImportRegex  = regexp.MustCompile(`(?m)^\s*import\s+(static\s+)?([A-Za-z0-9_.]+(?:\.\*)?)(?:\s+as\s+[A-Za-z0-9_]+)?\s*;?`)
	dartURIRegex     = regexp.MustCompile(`(?m)^\s*(?:import|export|part)\s+(?:[^;]*?\s+from\s+)?['"]([^'"]+)['"]`)
	zigImportRegex   = regexp.MustCompile(`@import\s*\(\s*["']([^"']+)["']\s*\)`)
)

// Imports returns local import targets for files handled by the otherDriver
// (C/C++ local headers, Ruby require_relative, PHP relative includes).
func (otherDriver) Imports(path, moduleRoot string, content []byte) []string {
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
	case ".java", ".kt", ".kts":
		targets = append(targets, javaTargets(dir, content)...)
	case ".dart":
		for _, match := range dartURIRegex.FindAllSubmatch(content, -1) {
			uri := string(match[1])
			switch {
			case strings.HasPrefix(uri, "./"), strings.HasPrefix(uri, "../"):
				targets = append(targets, filepath.ToSlash(filepath.Clean(filepath.Join(dir, uri))))
			case strings.HasPrefix(uri, "package:") && moduleRoot != "":
				pkgPath := strings.TrimPrefix(uri, "package:")
				pkgName, rest, ok := strings.Cut(pkgPath, "/")
				if ok && pkgName == moduleRoot {
					targets = append(targets, filepath.ToSlash(filepath.Join("lib", rest)))
				}
			}
		}
	case ".zig":
		for _, match := range zigImportRegex.FindAllSubmatch(content, -1) {
			name := string(match[1])
			if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") || strings.HasSuffix(name, ".zig") {
				targets = append(targets, filepath.ToSlash(filepath.Clean(filepath.Join(dir, name))))
			}
		}
	}
	return targets
}

func isUpperInitial(s string) bool {
	if len(s) == 0 {
		return false
	}
	r := rune(s[0])
	return r >= 'A' && r <= 'Z'
}

// findSourceRoot determines the source root for a JVM/Kotlin file when package
// declarations and directory structures diverge (common in Kotlin Multiplatform),
// looking for source set markers like /kotlin, /java, or /src/<variant>.
func findSourceRoot(dir string, pkgSegs []string) string {
	dirSegs := strings.Split(dir, "/")
	// Fallback for decoupled package directories (common in KMP):
	// Look for source root directory like /kotlin or /java in dir path
	for i := len(dirSegs) - 1; i >= 0; i-- {
		seg := dirSegs[i]
		if seg == "kotlin" || seg == "java" {
			return strings.Join(dirSegs[:i+1], "/")
		}
	}
	// Fallback for /src/<variant>
	for i := len(dirSegs) - 1; i >= 0; i-- {
		if dirSegs[i] == "src" {
			if i+2 < len(dirSegs) {
				return strings.Join(dirSegs[:i+3], "/")
			}
			if i+1 < len(dirSegs) {
				return strings.Join(dirSegs[:i+2], "/")
			}
		}
	}
	return ""
}

// javaTargets resolves JVM imports to repo-relative paths using the file's
// own package declaration as an anchor. The trailing directory segments of
// the importing file typically match the trailing segments of its package
// (e.g. src/main/java/com/example/demo ↔ com.example.demo); when packages are
// decoupled (e.g. in Kotlin Multiplatform), source root fallbacks are used.
func javaTargets(dir string, content []byte) []string {
	pkgMatch := javaPackageRegex.FindSubmatch(content)
	if pkgMatch == nil {
		return nil
	}
	pkgSegs := strings.Split(string(pkgMatch[1]), ".")
	dirSegs := strings.Split(dir, "/")
	anchor := 0
	for anchor < len(dirSegs) && anchor < len(pkgSegs) &&
		dirSegs[len(dirSegs)-1-anchor] == pkgSegs[len(pkgSegs)-1-anchor] {
		anchor++
	}
	var root string
	if anchor > 0 {
		root = strings.Join(dirSegs[:len(dirSegs)-anchor], "/")
	} else {
		root = findSourceRoot(dir, pkgSegs)
		if root == "" {
			return nil
		}
	}

	seen := make(map[string]bool)
	var targets []string
	addTarget := func(t string) {
		clean := filepath.ToSlash(filepath.Clean(t))
		if clean != "." && clean != "" && !seen[clean] {
			seen[clean] = true
			targets = append(targets, clean)
		}
	}

	for _, m := range javaImportRegex.FindAllSubmatch(content, -1) {
		imp := string(m[2])
		if !strings.Contains(imp, ".") || strings.HasPrefix(imp, "java.") {
			continue
		}
		segs := strings.Split(imp, ".")
		isStatic := len(m[1]) > 0
		drop := 1
		if isStatic {
			drop = 2
		} else if len(segs) > 2 && isUpperInitial(segs[len(segs)-2]) {
			// Member of class/companion object/enum (e.g. pkg.Status.ACTIVE or
			// pkg.Config.TIMEOUT): drop both member and class to reach package.
			drop = 2
		}
		if segs[len(segs)-1] == "*" {
			drop = 1
		}
		if len(segs) <= drop {
			continue
		}
		pkgPath := strings.Join(segs[:len(segs)-drop], "/")
		addTarget(root + "/" + pkgPath)
		if anchor == 0 {
			if root != dir {
				addTarget(root)
			}
			addTarget(dir)
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
	case ".dart":
		return types.Dart
	case ".zig":
		return types.Zig
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

func isJVMTest(path, base, ext string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	lowerBase := strings.ToLower(base)
	if strings.HasSuffix(lowerBase, "test"+ext) || strings.HasSuffix(lowerBase, "tests"+ext) || strings.HasSuffix(lowerBase, "it"+ext) || strings.HasSuffix(lowerBase, "spec"+ext) {
		return true
	}
	normPath := "/" + strings.TrimPrefix(lowerPath, "/")
	testDirs := []string{
		"/src/test/",
		"/src/androidtest/",
		"/src/androidunittest/",
		"/commontest/",
		"/jvmtest/",
		"/iostest/",
		"/desktoptest/",
	}
	for _, td := range testDirs {
		if strings.Contains(normPath, td) {
			return true
		}
	}
	// Check build-variant tests like /src/testDebug/, /src/testRelease/
	if idx := strings.Index(normPath, "/src/test"); idx != -1 {
		rest := normPath[idx+len("/src/test"):]
		if strings.Contains(rest, "/") {
			return true
		}
	}
	return false
}

func jvmFile(path, base, label, ext string) types.Classification {
	lowerBase := strings.ToLower(base)
	if isJVMTest(path, base, ext) {
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.90, Reason: label + " test file"}
	}
	if lowerBase == "application"+ext || lowerBase == "main"+ext {
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: label + " entrypoint"}
	}
	if strings.HasSuffix(lowerBase, "controller"+ext) || strings.HasSuffix(lowerBase, "service"+ext) || strings.HasSuffix(lowerBase, "api"+ext) {
		return types.Classification{Role: types.RoleAPI, Adjustment: 0.10, Confidence: 0.75, Reason: label + " API layer"}
	}
	if strings.HasSuffix(lowerBase, "repository"+ext) || strings.HasSuffix(lowerBase, "dao"+ext) || strings.HasSuffix(lowerBase, "database"+ext) {
		return types.Classification{Role: types.RoleAPI, Adjustment: 0.10, Confidence: 0.85, Reason: label + " data layer"}
	}
	if strings.HasSuffix(lowerBase, "entity"+ext) {
		return types.Classification{Role: types.RoleSchema, Adjustment: 0.08, Confidence: 0.85, Reason: label + " entity schema"}
	}
	if strings.HasSuffix(lowerBase, "screen"+ext) || strings.HasSuffix(lowerBase, "viewmodel"+ext) || strings.HasSuffix(lowerBase, "activity"+ext) || strings.HasSuffix(lowerBase, "dialog"+ext) || strings.HasSuffix(lowerBase, "bottomsheet"+ext) {
		return types.Classification{Role: types.RoleImpl, Adjustment: 0.10, Confidence: 0.85, Reason: label + " UI/State component"}
	}
	return conventionByName(base, label)
}
