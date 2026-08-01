package language

import "strings"

// RuleJava filters Java/Kotlin build artifacts and wrappers.
func RuleJava(path, filename string, content []byte) bool {
	if filename == "gradle-wrapper.jar" || filename == "gradlew.bat" {
		return true
	}
	if strings.HasSuffix(filename, ".class") || strings.HasSuffix(filename, ".jar") || strings.HasSuffix(filename, ".aar") {
		return true
	}
	return false
}
