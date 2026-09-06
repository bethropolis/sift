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
		{"server.js", RoleEntrypoint},
		{"src/main.ts", RoleEntrypoint},
		{"src/middleware.ts", RoleEntrypoint},
		{"src/app/routes.js", RoleEntrypoint},
		{"src/app/page.jsx", RoleEntrypoint},
		{"config/routes.rb", RoleAPI},
		{"settings.gradle", RoleConfig},
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

func TestResolveImports(t *testing.T) {
	mkExists := func(paths ...string) func(string) bool {
		set := map[string]bool{}
		for _, p := range paths {
			set[p] = true
		}
		return func(p string) bool { return set[p] }
	}
	tests := []struct {
		name   string
		path   string
		mod    string
		src    string
		exists func(string) bool
		want   []string
	}{
		{
			name:   "go module import resolves to package dir",
			path:   "main.go",
			mod:    "example.com/repo",
			src:    "package main\nimport \"example.com/repo/internal/app\"\n",
			exists: mkExists("main.go", "internal/app/app.go", "internal/app"),
			want:   []string{"internal/app"},
		},
		{
			name:   "go stdlib and uncollected omitted",
			path:   "main.go",
			mod:    "example.com/repo",
			src:    "package main\nimport (\n\"fmt\"\n\"example.com/repo/missing\"\n)\n",
			exists: mkExists("main.go"),
			want:   nil,
		},
		{
			name:   "js relative with extension fallback",
			path:   "app/page.jsx",
			src:    "import x from './local';\nimport React from 'react';\n",
			exists: mkExists("app/page.jsx", "app/local.ts"),
			want:   []string{"app/local.ts"},
		},
		{
			name:   "js directory index fallback",
			path:   "app/page.js",
			src:    "import p from './pkg';\n",
			exists: mkExists("app/page.js", "app/pkg/index.js"),
			want:   []string{"app/pkg/index.js"},
		},
		{
			name:   "ts type import and alias",
			path:   "src/app/page.tsx",
			src:    "import type { T } from './types';\nimport { api } from '~/lib/api';\n",
			exists: mkExists("src/app/page.tsx", "src/app/types.ts", "lib/api.ts"),
			want:   []string{"src/app/types.ts", "lib/api.ts"},
		},
		{
			name:   "python generic fallback passes package dir through",
			path:   "pkg/mod.py",
			src:    "from . import helper\n",
			exists: mkExists("pkg/mod.py", "pkg"),
			want:   []string{"pkg"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveImports(tc.path, tc.mod, []byte(tc.src), tc.exists)
			if len(got) != len(tc.want) {
				t.Fatalf("ResolveImports(%q) = %v, want %v", tc.path, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ResolveImports(%q) = %v, want %v", tc.path, got, tc.want)
				}
			}
		})
	}
}
func TestImportsLocalResolution(t *testing.T) {
	tests := []struct {
		name string
		path string
		mod  string
		src  string
		want []string
	}{
		{
			name: "java same package",
			path: "services/api/src/main/java/com/example/api/Config.java",
			src:  "package com.example.api;\nimport com.example.api.sub.Service;\nimport java.util.List;\n",
			want: []string{"services/api/src/main/java/com/example/api/sub"},
		},
		{
			name: "kotlin static and wildcard",
			path: "app/src/main/kotlin/com/example/app/Main.kt",
			src:  "package com.example.app\nimport com.example.app.ConfigKt\nimport static com.example.app.Config.TIMEOUT\n",
			want: []string{"app/src/main/kotlin/com/example/app"},
		},
		{
			name: "rust super and mod",
			path: "src/db/pool.rs",
			src:  "use super::config::Pool;\nmod tests;\n",
			want: []string{"src/config", "src/config/Pool", "src/db/tests.rs", "src/db/tests/mod.rs"},
		},
		{
			name: "rust crate root in workspace member",
			path: "crates/auth/src/main.rs",
			src:  "use crate::config;\n",
			want: []string{"crates/auth/config"},
		},
		{
			name: "js workspace alias",
			path: "app/page.jsx",
			src:  "import Button from '@/components/Button';\nimport x from './local';\nimport React from 'react';\n",
			want: []string{"components/Button", "app/local"},
		},
		{
			name: "ts workspace alias",
			path: "src/app/page.tsx",
			src:  "import { api } from '~/lib/api';\nimport { y } from '../util';\n",
			want: []string{"lib/api", "src/util"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Imports(tc.path, tc.mod, []byte(tc.src))
			if len(got) != len(tc.want) {
				t.Fatalf("Imports(%q) = %v, want %v", tc.path, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("Imports(%q) = %v, want %v", tc.path, got, tc.want)
				}
			}
		})
	}
}
