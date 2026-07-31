// Package secrets detects and redacts credential-like strings before output.
package secrets

import (
	"regexp"
)

// Detection describes a single secret found in a file.
type Detection struct {
	RuleName string
	Match    string
}

// Rule is a named regex used to find secrets.
type Rule struct {
	Name  string
	Regex *regexp.Regexp
}

// rules is a curated set of gitleaks-compatible patterns covering common
// credential formats.
var rules = []Rule{
	{Name: "AWS Access Key", Regex: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{Name: "AWS Secret Key", Regex: regexp.MustCompile(`(?i)aws(.{0,20})?['"][0-9a-zA-Z/+]{40}['"]`)},
	{Name: "GitHub Token", Regex: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,255}`)},
	{Name: "Slack Token", Regex: regexp.MustCompile(`xox[baprs]-[0-9a-zA-Z-]{10,48}`)},
	{Name: "Google API Key", Regex: regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},
	{Name: "Stripe Secret Key", Regex: regexp.MustCompile(`(?i)(sk|pk)_(test|live)_[0-9a-zA-Z]{10,}`)},
	{Name: "JWT", Regex: regexp.MustCompile(`eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_.+/=]+`)},
	{Name: "Private Key", Regex: regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`)},
	{Name: "Bearer Token", Regex: regexp.MustCompile(`(?i)bearer\s+[a-z0-9._\-~+/]+=*`)},
}

// Scanner scans content against the secret rules.
type Scanner struct{}

// New returns a Scanner.
func New() *Scanner {
	return &Scanner{}
}

// Scan returns all secrets detected in content.
func (s *Scanner) Scan(content []byte) []Detection {
	var detections []Detection
	for _, rule := range rules {
		for _, match := range rule.Regex.FindAll(content, -1) {
			detections = append(detections, Detection{RuleName: rule.Name, Match: string(match)})
		}
	}
	return detections
}

// Redact replaces detected secrets with a placeholder and returns the
// sanitized content along with the detections.
func (s *Scanner) Redact(content []byte) ([]byte, []Detection) {
	var detections []Detection
	redacted := string(content)
	for _, rule := range rules {
		matches := rule.Regex.FindAllStringIndex(redacted, -1)
		if len(matches) == 0 {
			continue
		}
		// Replace from the end so earlier indices stay valid.
		for i := len(matches) - 1; i >= 0; i-- {
			start, end := matches[i][0], matches[i][1]
			match := redacted[start:end]
			detections = append(detections, Detection{RuleName: rule.Name, Match: match})
			redacted = redacted[:start] + "[REDACTED_SECRET: " + rule.Name + "]" + redacted[end:]
		}
	}
	return []byte(redacted), detections
}
