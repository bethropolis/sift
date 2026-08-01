// Package language contains filename/content heuristics that identify
// generated, bundled, or lockfile artifacts that add little LLM context.
package language

// Rule checks if a path/filename/content matches a language-specific filter.
// The filename is passed lowercased and basenamed; path is lowercased with
// forward slashes.
type Rule func(path, filename string, content []byte) bool

// AllRules returns the full set of registered language rules.
func AllRules() []Rule {
	return []Rule{
		RuleGo,
		RuleRust,
		RuleJavaScript,
		RuleJava,
		RulePHP,
		RuleRuby,
	}
}
