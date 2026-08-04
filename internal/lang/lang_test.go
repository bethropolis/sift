package lang

import "testing"

func TestClassifyConventions(t *testing.T) {
	tests := []struct {
		path string
		role Role
	}{
		{"cmd/sift/main.go", RoleEntrypoint},
		{"internal/app/app_test.go", RoleTest},
		{"internal/app/service_mock.go", RoleMock},
		{"src/api.d.ts", RoleAPI},
		{"tests/test_auth.py", RoleTest},
		{"src/lib.rs", RoleAPI},
		{"migrations/001_init.sql", RoleSchema},
		{"README.md", RoleDocs},
		{"generated/client.gen.go", RoleGenerated},
		{"server.js", RoleImpl},
		{"src/main.ts", RoleEntrypoint},
		{"web.php", RoleImpl},
		{"MyApp.java", RoleImpl},
		{"unknown.xyz", RoleUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := Classify(tc.path).Role; got != tc.role {
				t.Fatalf("Classify(%q).Role = %q, want %q", tc.path, got, tc.role)
			}
		})
	}
}

func TestForPathAndShouldSkipSmart(t *testing.T) {
	if _, ok := ForPath("main.go"); !ok {
		t.Error("ForPath(main.go) should resolve")
	}
	if skip, _ := ShouldSkipSmart("go.sum", "go.sum", nil); !skip {
		t.Error("go.sum should be smart-skipped")
	}
	if skip, _ := ShouldSkipSmart("app.js", "app.js", []byte("function x(){}")); skip {
		t.Errorf("plain js should not be skipped")
	}
}
