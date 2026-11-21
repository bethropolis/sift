package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/denormal/go-gitignore"
)

// ModesConfig holds path-based compression and filtering rules.
type ModesConfig struct {
	Full       StringList `toml:"full"`
	Signatures StringList `toml:"signatures"`
	Outline    StringList `toml:"outline"`
	Skip       StringList `toml:"skip"`
}

// ModeDecision represents the compression mode assigned to a specific file.
type ModeDecision int

const (
	ModeDecisionDefault ModeDecision = iota
	ModeDecisionSkip
	ModeDecisionFull
	ModeDecisionOutline
	ModeDecisionSignatures
)

// ModesMatcher matches file paths against glob rules defined in ModesConfig.
type ModesMatcher struct {
	skip       gitignore.GitIgnore
	full       gitignore.GitIgnore
	outline    gitignore.GitIgnore
	signatures gitignore.GitIgnore
}

// NewModesMatcher constructs a matcher from ModesConfig. Returns nil if no rules are defined.
func NewModesMatcher(rootDir string, m ModesConfig) *ModesMatcher {
	if len(m.Skip) == 0 && len(m.Full) == 0 && len(m.Outline) == 0 && len(m.Signatures) == 0 {
		return nil
	}
	mm := &ModesMatcher{}
	if len(m.Skip) > 0 {
		mm.skip = gitignore.New(strings.NewReader(strings.Join([]string(m.Skip), "\n")), rootDir, nil)
	}
	if len(m.Full) > 0 {
		mm.full = gitignore.New(strings.NewReader(strings.Join([]string(m.Full), "\n")), rootDir, nil)
	}
	if len(m.Outline) > 0 {
		mm.outline = gitignore.New(strings.NewReader(strings.Join([]string(m.Outline), "\n")), rootDir, nil)
	}
	if len(m.Signatures) > 0 {
		mm.signatures = gitignore.New(strings.NewReader(strings.Join([]string(m.Signatures), "\n")), rootDir, nil)
	}
	return mm
}

// Match tests a path relative to the root directory and returns the assigned mode decision.
// Precedence order: skip > full > outline > signatures > default.
func (mm *ModesMatcher) Match(relPath string) ModeDecision {
	if mm == nil {
		return ModeDecisionDefault
	}
	rel := filepath.ToSlash(relPath)
	if mm.skip != nil {
		if match := mm.skip.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionSkip
		}
	}
	if mm.full != nil {
		if match := mm.full.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionFull
		}
	}
	if mm.outline != nil {
		if match := mm.outline.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionOutline
		}
	}
	if mm.signatures != nil {
		if match := mm.signatures.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionSignatures
		}
	}
	return ModeDecisionDefault
}

// ExportModes reduces per-file mode assignments to a minimal [modes] config.
// A directory whose eligible files all share one non-default mode collapses to
// "dir/**"; mixed directories fall back to exact file paths. Paths with the
// default (empty/full) mode are omitted.
func ExportModes(assignments map[string]string) ModesConfig {
	type entry struct {
		path string
		mode string
		emit bool
	}
	entries := make([]entry, 0, len(assignments))
	for path, raw := range assignments {
		path = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(path)), "./")
		if path == "" {
			continue
		}
		mode := canonicalExportMode(raw)
		if mode == "" {
			// Full is the default and omitted from the final TOML, but it must
			// still participate in directory uniformity: a folder mixing full
			// and signatures must not collapse to "dir/**" signatures.
			entries = append(entries, entry{path: path, mode: "full"})
			continue
		}
		entries = append(entries, entry{path: path, mode: mode, emit: true})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	// Collect mode uniformity and membership for every ancestor directory.
	// Include default/full files here so a mixed directory is never collapsed.
	type dirInfo struct {
		count int
		mode  string
		mixed bool
	}
	dirs := map[string]*dirInfo{}
	for _, e := range entries {
		for d := filepath.Dir(e.path); ; d = filepath.Dir(d) {
			d = filepath.ToSlash(d)
			info := dirs[d]
			if info == nil {
				info = &dirInfo{mode: e.mode}
				dirs[d] = info
			}
			info.count++
			if info.mode != e.mode {
				info.mixed = true
			}
			if d == "." || d == "/" || d == "" {
				break
			}
		}
	}

	dirNames := make([]string, 0, len(dirs))
	for d, info := range dirs {
		if d == "." || d == "" || info.mixed || info.count < 2 || info.mode == "full" {
			continue
		}
		dirNames = append(dirNames, d)
	}
	sort.Slice(dirNames, func(i, j int) bool {
		di, dj := strings.Count(dirNames[i], "/"), strings.Count(dirNames[j], "/")
		if di != dj {
			return di < dj
		}
		return dirNames[i] < dirNames[j]
	})

	var out ModesConfig
	collapsed := map[string]bool{}
	for _, dir := range dirNames {
		info := dirs[dir]
		busy := false
		for _, e := range entries {
			if e.emit && strings.HasPrefix(e.path, dir+"/") && collapsed[e.path] {
				busy = true
				break
			}
		}
		if busy {
			continue
		}
		for _, e := range entries {
			if e.emit && strings.HasPrefix(e.path, dir+"/") {
				collapsed[e.path] = true
			}
		}
		addModePattern(&out, info.mode, dir+"/**")
	}

	for _, e := range entries {
		if !e.emit || collapsed[e.path] {
			continue
		}
		addModePattern(&out, e.mode, e.path)
	}
	sort.Strings(out.Full)
	sort.Strings(out.Outline)
	sort.Strings(out.Signatures)
	sort.Strings(out.Skip)
	return out
}

func canonicalExportMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "full", "default", "include":
		return ""
	case "sigs", "signatures", "signature":
		return "signatures"
	case "outline", "outlines":
		return "outline"
	case "skip", "exclude", "ignored":
		return "skip"
	}
	return ""
}

func addModePattern(m *ModesConfig, mode, pattern string) {
	switch mode {
	case "full":
		m.Full = append(m.Full, pattern)
	case "outline":
		m.Outline = append(m.Outline, pattern)
	case "signatures":
		m.Signatures = append(m.Signatures, pattern)
	case "skip":
		m.Skip = append(m.Skip, pattern)
	}
}

// FormatModesTOML renders only the non-empty [modes] lists, deterministically
// ordered, so the result can be pasted into .sift.toml or committed as-is.
func FormatModesTOML(m ModesConfig) string {
	var b strings.Builder
	writeList := func(name string, values []string) {
		if len(values) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s = [\n", name)
		for _, value := range values {
			fmt.Fprintf(&b, "  %q,\n", value)
		}
		b.WriteString("]\n")
	}
	if len(m.Full)+len(m.Outline)+len(m.Signatures)+len(m.Skip) == 0 {
		return ""
	}
	b.WriteString("[modes]\n")
	writeList("full", m.Full)
	writeList("outline", m.Outline)
	writeList("signatures", m.Signatures)
	writeList("skip", m.Skip)
	return b.String()
}
