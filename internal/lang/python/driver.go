package langpy

import (
	"strings"

	"github.com/bethropolis/sift/internal/lang/registry"
	"github.com/bethropolis/sift/internal/lang/types"
)

type pyDriver struct{}

func init() { registry.Register(pyDriver{}) }

func (pyDriver) ID() types.ID         { return types.Python }
func (pyDriver) Name() string         { return "Python" }
func (pyDriver) Extensions() []string { return []string{".py", ".pyi"} }
func (pyDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	return false, ""
}
func (pyDriver) Classify(path, filename string) types.Classification {
	switch {
	case strings.HasPrefix(filename, "test_") || strings.HasSuffix(filename, "_test.py") || strings.Contains(path, "/tests/"):
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "Python test file"}
	case filename == "__main__.py" || filename == "manage.py" || filename == "cli.py":
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Python entrypoint"}
	case filename == "__init__.py":
		return types.Classification{Role: types.RoleAPI, Adjustment: 0.10, Confidence: 0.85, Reason: "Python package API"}
	case filename == "conftest.py":
		return types.Classification{Role: types.RoleFixture, Adjustment: -0.06, Confidence: 0.95, Reason: "pytest fixture"}
	}
	return types.Classification{Role: types.RoleImpl, Confidence: 0.50, Reason: "Python source"}
}
