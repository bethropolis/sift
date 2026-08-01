package language

// RulePHP filters PHP composer lockfiles.
func RulePHP(path, filename string, content []byte) bool {
	return filename == "composer.lock"
}
