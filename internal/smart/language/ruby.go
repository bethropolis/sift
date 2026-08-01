package language

// RuleRuby filters Ruby Gemfile lockfiles.
func RuleRuby(path, filename string, content []byte) bool {
	return filename == "gemfile.lock"
}
