package language

import "strings"

func pythonFile(path, base string) Classification {
	switch {
	case strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") || strings.Contains(path, "/tests/"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "Python test file"}
	case base == "__main__.py" || base == "manage.py" || base == "cli.py":
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Python entrypoint"}
	case base == "__init__.py":
		return Classification{Role: RoleAPI, Adjustment: 0.10, Confidence: 0.85, Reason: "Python package API"}
	case base == "conftest.py":
		return Classification{Role: RoleFixture, Adjustment: -0.06, Confidence: 0.95, Reason: "pytest fixture"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "Python source"}
}
