package secrets

import (
	"math"
	"regexp"
	"strings"
)

// ruleFilter applies the gitleaks post-match logic that the generated rule
// table cannot express: which capture group holds the secret, an optional
// Shannon-entropy floor, and stopwords for placeholder values. The generated
// files only carry name/keywords/regex, so filters live separately here and
// are keyed by rule name.
type ruleFilter struct {
	// SecretGroup is the submatch index holding the secret. The whole match is
	// replaced when zero (the default gitleaks behavior for most rules).
	SecretGroup int
	// MinEntropy rejects candidates below this Shannon entropy (bits/byte).
	MinEntropy float64
	// StopWords are case-insensitive substrings that mark a placeholder.
	StopWords []string
}

var ruleFilters = map[string]ruleFilter{
	// generic-api-key is intentionally broad: it matches any assignment whose
	// name contains "token", "key", "secret", etc. Without gitleaks' entropy
	// and stopword filters it redacts ordinary code like
	// "dst.TokenizeModel = src.TokenizeModel". Filter on capture group 1 so
	// only the value is replaced, not the surrounding statement.
	"generic-api-key": {
		SecretGroup: 1,
		MinEntropy:  3.5,
		StopWords: []string{
			"example", "sample", "placeholder", "changeme", "change_me",
			"default", "dummy", "lorem", "ipsum", "password", "passwd",
			"credential", "credentials", "secret", "token", "username",
			"user", "admin", "test", "testing", "xxxx", "0000", "1234",
			"abcdef",
		},
	},
}

// identChain matches a dotted identifier path such as "src.TokenizeModel" or
// "types.RoleAPI". Real provider keys are never shaped like a Go selector.
var identChain = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+$`)

// wordShapes match identifiers, environment-variable names, CLI flags, and
// prose compounds. They have enough entropy to pass the generic threshold but
// are never secrets ("GIT_CONFIG_VALUE_", "signature-only", "fallback-mode").
var (
	upperIdent = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	kebabWord  = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)
	snakeWord  = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)+$`)
)

// accept reports whether a candidate secret survives the filter.
func (f ruleFilter) accept(candidate []byte) bool {
	if len(candidate) == 0 {
		return false
	}
	s := string(candidate)
	lower := strings.ToLower(s)
	for _, word := range f.StopWords {
		if strings.Contains(lower, word) {
			return false
		}
	}
	if identChain.MatchString(s) || upperIdent.MatchString(s) ||
		kebabWord.MatchString(s) || snakeWord.MatchString(s) {
		return false
	}
	// A generic key with no digit or symbol is almost always an identifier or
	// a dictionary word. Require at least one non-letter character.
	hasSymbol := false
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			hasSymbol = true
			break
		}
	}
	if !hasSymbol {
		return false
	}
	return shannonEntropy(candidate) >= f.MinEntropy
}

// shannonEntropy returns the Shannon entropy of b in bits per byte.
func shannonEntropy(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	var counts [256]int
	for _, c := range b {
		counts[c]++
	}
	n := float64(len(b))
	var entropy float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		entropy -= p * math.Log2(p)
	}
	return entropy
}
