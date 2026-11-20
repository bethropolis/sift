package langrs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceCrateImportsResolve(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", `[workspace]
members = ["crates/*"]
`)
	write("crates/core/Cargo.toml", `[package]
name = "core-lib"
version = "0.1.0"
`)
	write("crates/core/src/lib.rs", "pub struct Core;\n")
	write("crates/app/Cargo.toml", `[package]
name = "app"
version = "0.1.0"
`)
	write("crates/app/src/lib.rs", "use core_lib::Core;\n")

	content := []byte("use core_lib::Core;\n")
	exists := func(p string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		return err == nil
	}
	got := rsDriver{}.ResolveImportsInRoot("crates/app/src/lib.rs", root, "", content, exists)
	found := false
	for _, p := range got {
		if p == "crates/core/src/lib.rs" {
			found = true
		}
	}
	if !found {
		t.Fatalf("workspace crate root not resolved: %v", got)
	}
}
