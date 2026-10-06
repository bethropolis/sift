package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// maxConfigFileBytes caps .sift.toml / config.toml reads. Real configs are a
// few kilobytes; anything larger is either corruption or abuse, and the
// parser must never allocate unbounded memory on a hostile repo's file.
const maxConfigFileBytes = 1 << 20 // 1 MiB

// readConfigFileCapped reads path with the size cap applied, returning a
// zero value when the file does not exist.
func readConfigFileCapped(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxConfigFileBytes {
		return nil, fmt.Errorf("config file %q exceeds %d bytes", path, maxConfigFileBytes)
	}
	// Bound the read itself: the size could change between Stat and Read.
	data := make([]byte, 0, min(info.Size()+1, maxConfigFileBytes+1))
	buf := make([]byte, 32*1024)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			if len(data) > maxConfigFileBytes {
				return nil, fmt.Errorf("config file %q exceeds %d bytes", path, maxConfigFileBytes)
			}
		}
		if readErr != nil {
			break
		}
	}
	return data, nil
}

// escapesRoot reports whether p (a config-declared path) resolves outside
// root: absolute paths and ..-traversals are both rejected.
func escapesRoot(root, p string) bool {
	if p == "" {
		return false
	}
	if filepath.IsAbs(filepath.FromSlash(p)) {
		return true
	}
	clean := filepath.Clean(filepath.FromSlash(p))
	return clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

// parseConfigBytes decodes a v2 config file from memory for inspection.
func parseConfigBytes(data []byte) (configFile, error) {
	var cf configFile
	if err := toml.Unmarshal(data, &cf); err != nil {
		return cf, err
	}
	if cf.Profiles == nil {
		cf.Profiles = map[string]Profile{}
	}
	if cf.Prompts == nil {
		cf.Prompts = map[string]PromptRef{}
	}
	return cf, nil
}

// CheckProjectConfig inspects the .sift.toml in root (the same file
// ResolveConfig would load) and reports declarations that would read or
// write outside the project: prompt_file / prompts.*.file / ui_theme_file
// reads and output writes. CLI commands keep their historical behavior; the
// serve frontend refuses projects whose configs escape, with a generic 403
// that names nothing about the filesystem.
func CheckProjectConfig(root string) []string {
	var issues []string
	path := filepath.Join(root, ".sift.toml")
	data, err := readConfigFileCapped(path)
	if err != nil || len(data) == 0 {
		if err != nil {
			issues = append(issues, "unreadable .sift.toml")
		}
		return issues
	}
	cf, err := parseConfigBytes(data)
	if err != nil {
		issues = append(issues, "unparsable .sift.toml")
		return issues
	}
	check := func(where, p string) {
		if escapesRoot(root, p) {
			issues = append(issues, where+" escapes the project root")
		}
	}
	if cf.Sift != nil {
		check("[sift] prompt_file", cf.Sift.PromptFile)
		check("[sift] output", cf.Sift.Output)
		check("[sift] ui_theme_file", cf.Sift.UIThemeFile)
	}
	for name, p := range cf.Profiles {
		check("profile "+name+" prompt_file", p.PromptFile)
		check("profile "+name+" output", p.Output)
	}
	for _, t := range cf.Targets {
		check("target "+t.Name+" prompt_file", t.PromptFile)
		check("target "+t.Name+" output", t.Output)
	}
	for name, p := range cf.Prompts {
		check("prompt "+name+" file", p.File)
	}
	return issues
}
