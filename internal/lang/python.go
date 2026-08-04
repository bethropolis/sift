package lang

import "strings"

type pyDriver struct{}

func init() { Register(pyDriver{}) }

func (pyDriver) ID() ID               { return Python }
func (pyDriver) Name() string         { return "Python" }
func (pyDriver) Extensions() []string { return []string{".py", ".pyi"} }
func (pyDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	return false, ""
}
func (pyDriver) Classify(path, filename string) Classification {
	switch {
	case strings.HasPrefix(filename, "test_") || strings.HasSuffix(filename, "_test.py") || strings.Contains(path, "/tests/"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "Python test file"}
	case filename == "__main__.py" || filename == "manage.py" || filename == "cli.py":
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Python entrypoint"}
	case filename == "__init__.py":
		return Classification{Role: RoleAPI, Adjustment: 0.10, Confidence: 0.85, Reason: "Python package API"}
	case filename == "conftest.py":
		return Classification{Role: RoleFixture, Adjustment: -0.06, Confidence: 0.95, Reason: "pytest fixture"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "Python source"}
}
