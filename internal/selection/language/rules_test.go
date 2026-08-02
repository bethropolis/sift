package language

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
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := Classify(tc.path).Role; got != tc.role {
				t.Fatalf("Classify(%q).Role = %q, want %q", tc.path, got, tc.role)
			}
		})
	}
}
