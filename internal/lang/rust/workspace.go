package langrs

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var cargoWorkspaceCache sync.Map // absolute root -> map[crateName]crateDir

var (
	cargoNameRe    = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)
	cargoMembersRe = regexp.MustCompile(`(?s)\bmembers\s*=\s*\[([^\]]*)\]`)
	cargoMemberRe  = regexp.MustCompile(`"([^"]+)"`)
)

// workspaceCrates maps workspace crate names to their repo-relative
// directories, so `use other_crate::foo` can resolve across members.
func workspaceCrates(root string) map[string]string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	if cached, ok := cargoWorkspaceCache.Load(abs); ok {
		return cached.(map[string]string)
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		return nil
	}
	defer r.Close()

	crates := map[string]string{}
	addCrate := func(dir string) {
		name := cargoPackageName(r, dir)
		if name == "" {
			return
		}
		rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(dir)))
		crates[name] = rel
		// Rust crate identifiers use underscores where Cargo package names
		// use hyphens.
		crates[strings.ReplaceAll(name, "-", "_")] = rel
	}
	addCrate(".")

	data, err := r.ReadFile("Cargo.toml")
	if err == nil {
		if m := cargoMembersRe.FindSubmatch(data); m != nil {
			for _, mm := range cargoMemberRe.FindAllSubmatch(m[1], -1) {
				member := filepath.ToSlash(string(mm[1]))
				if strings.ContainsAny(member, "*?[") {
					matches, _ := fs.Glob(r.FS(), member)
					for _, match := range matches {
						addCrate(match)
					}
					continue
				}
				addCrate(member)
			}
		}
	}
	cargoWorkspaceCache.Store(abs, crates)
	return crates
}

func cargoPackageName(r *os.Root, dir string) string {
	data, err := r.ReadFile(filepath.Join(filepath.FromSlash(dir), "Cargo.toml"))
	if err != nil {
		return ""
	}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[] ")
			continue
		}
		if section == "package" {
			if m := cargoNameRe.FindStringSubmatch(line); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

// ResolveImportsInRoot resolves Rust imports including cross-crate workspace
// paths. Workspace members are discovered from the root Cargo.toml.
func (rsDriver) ResolveImportsInRoot(path, root, _ string, content []byte, exists func(string) bool) []string {
	return rsResolve(path, content, exists, workspaceCrates(root))
}

func rsResolve(path string, content []byte, exists func(string) bool, workspace map[string]string) []string {
	dir := filepath.ToSlash(filepath.Dir(path))
	crateDir := crateBaseDir(path)
	crateSrc := filepath.ToSlash(filepath.Join(crateDir, "src"))
	if crateDir == "" {
		crateSrc = "src"
	}
	var out []string
	seen := map[string]bool{}
	add := func(base string) {
		for _, cand := range rustCandidates(base) {
			if !seen[cand] && exists(cand) {
				seen[cand] = true
				out = append(out, cand)
			}
		}
	}

	for _, m := range rsUse.FindAllSubmatch(content, -1) {
		use := string(m[1])
		var from string
		switch {
		case strings.HasPrefix(use, "crate::"):
			from = crateSrc
			use = strings.TrimPrefix(use, "crate::")
		case strings.HasPrefix(use, "self::"):
			from = dir
			use = strings.TrimPrefix(use, "self::")
		case strings.HasPrefix(use, "super::"):
			from = dir
			for strings.HasPrefix(use, "super::") {
				use = strings.TrimPrefix(use, "super::")
				from = filepath.ToSlash(filepath.Dir(from))
			}
		default:
			first := use
			if idx := strings.Index(first, "::"); idx >= 0 {
				first = first[:idx]
			}
			memberDir, ok := workspace[first]
			if !ok {
				continue // external crate
			}
			from = filepath.ToSlash(filepath.Join(memberDir, "src"))
			use = strings.TrimPrefix(use, first)
			use = strings.TrimPrefix(use, "::")
			// The crate root is lib.rs/main.rs under src/.
			add(from)
		}
		segments := strings.Split(use, "::")
		for i := 1; i <= len(segments); i++ {
			if segments[0] == "" {
				break
			}
			add(filepath.ToSlash(filepath.Clean(filepath.Join(from, filepath.Join(segments[:i]...)))))
		}
	}

	for _, m := range rsMod.FindAllSubmatch(content, -1) {
		mod := string(m[1])
		add(filepath.ToSlash(filepath.Join(dir, mod)))
	}
	return out
}

// rustCandidates mirrors Rust's module layout: foo.rs or foo/mod.rs for a
// module named foo, and the bare directory for package-node fan-in.
func rustCandidates(base string) []string {
	return []string{
		base,
		base + ".rs",
		filepath.ToSlash(filepath.Join(base, "mod.rs")),
		filepath.ToSlash(filepath.Join(base, "lib.rs")),
		filepath.ToSlash(filepath.Join(base, "main.rs")),
	}
}
