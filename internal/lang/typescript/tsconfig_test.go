package langts

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTS(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveImportsHonorsTSConfigAliases(t *testing.T) {
	root := t.TempDir()
	writeTS(t, root, "tsconfig.json", `{
  // JSONC comments and trailing commas must be tolerated
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@app/*": ["src/app/*"],
      "$lib/*": ["src/lib/*"],
    },
  },
}`)
	writeTS(t, root, "src/app/y.ts", "export const y = 1;\n")
	writeTS(t, root, "src/lib/z.ts", "export const z = 2;\n")
	writeTS(t, root, "src/feature.ts", "import { y } from '@app/y';\nimport { z } from '$lib/z';\n")

	exists := func(p string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		return err == nil
	}
	got := tsDriver{}.ResolveImportsInRoot("src/feature.ts", root, "", []byte("import { y } from '@app/y';\nimport { z } from '$lib/z';\n"), exists)
	want := map[string]bool{"src/app/y.ts": true, "src/lib/z.ts": true}
	if len(got) != len(want) {
		t.Fatalf("resolved = %v, want %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("unexpected resolution %q (all: %v)", p, got)
		}
	}
}

func TestResolveImportsFollowsBarrelExports(t *testing.T) {
	root := t.TempDir()
	writeTS(t, root, "src/index.ts", "export * from './a';\nexport { b } from './b';\n")
	writeTS(t, root, "src/a.ts", "export const a = 1;\n")
	writeTS(t, root, "src/b.ts", "export const b = 2;\n")

	exists := func(p string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		return err == nil
	}
	content := []byte("export * from './a';\nexport { b } from './b';\n")
	got := tsDriver{}.ResolveImports("src/index.ts", "", content, exists)
	want := map[string]bool{"src/a.ts": true, "src/b.ts": true}
	if len(got) != len(want) {
		t.Fatalf("resolved = %v, want %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("unexpected resolution %q (all: %v)", p, got)
		}
	}
}
