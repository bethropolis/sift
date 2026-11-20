package langts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// tsConfig is the subset of tsconfig/jsconfig that affects module resolution.
type tsConfig struct {
	baseURL string              // repo-relative directory
	paths   map[string][]string // alias pattern -> repo-relative target patterns
}

type rawTSConfig struct {
	Extends         string `json:"extends"`
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

var tsConfigCache sync.Map // absolute config path -> *tsConfig

// loadTSConfig finds the nearest tsconfig.json/jsconfig.json above relPath and
// flattens its baseUrl/paths into repo-relative patterns. Local "extends" are
// merged so a shared base config still contributes aliases.
func loadTSConfig(root, relPath string) *tsConfig {
	dir := filepath.ToSlash(filepath.Dir(relPath))
	for {
		for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
			rel := filepath.ToSlash(filepath.Join(dir, name))
			if dir == "." {
				rel = name
			}
			abs := filepath.Join(root, filepath.FromSlash(rel))
			if info, err := os.Stat(abs); err == nil && !info.IsDir() {
				return loadTSConfigFile(root, rel, abs, 0)
			}
		}
		if dir == "." || dir == "" {
			return nil
		}
		dir = filepath.ToSlash(filepath.Dir(dir))
	}
}

func loadTSConfigFile(root, rel, abs string, depth int) *tsConfig {
	if cached, ok := tsConfigCache.Load(abs); ok {
		return cached.(*tsConfig)
	}
	if depth > 4 {
		return nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	var raw rawTSConfig
	if err := json.Unmarshal(stripJSONC(data), &raw); err != nil {
		return nil
	}
	base := &tsConfig{}
	if raw.Extends != "" && (strings.HasPrefix(raw.Extends, ".") || strings.HasPrefix(raw.Extends, "/")) {
		parentRel := strings.TrimSuffix(filepath.ToSlash(filepath.Join(filepath.Dir(rel), raw.Extends)), ".json") + ".json"
		parentAbs := filepath.Join(root, filepath.FromSlash(parentRel))
		if _, err := os.Stat(parentAbs); err == nil {
			base = loadTSConfigFile(root, parentRel, parentAbs, depth+1)
		}
	}
	cfg := &tsConfig{baseURL: base.baseURL, paths: map[string][]string{}}
	for k, v := range base.paths {
		cfg.paths[k] = append([]string(nil), v...)
	}

	configDir := filepath.ToSlash(filepath.Dir(rel))
	if configDir == "." {
		configDir = ""
	}
	baseDir := configDir
	if raw.CompilerOptions.BaseURL != "" {
		baseDir = filepath.ToSlash(filepath.Clean(filepath.Join(configDir, raw.CompilerOptions.BaseURL)))
	}
	cfg.baseURL = strings.TrimPrefix(baseDir, "./")
	for pattern, targets := range raw.CompilerOptions.Paths {
		out := make([]string, 0, len(targets))
		for _, target := range targets {
			out = append(out, filepath.ToSlash(filepath.Clean(filepath.Join(baseDir, target))))
		}
		cfg.paths[pattern] = out
	}
	tsConfigCache.Store(abs, cfg)
	return cfg
}

// match returns the repo-relative candidate bases for a bare specifier.
func (c *tsConfig) match(spec string) []string {
	if c == nil {
		return nil
	}
	var out []string
	for pattern, targets := range c.paths {
		star := strings.Index(pattern, "*")
		var capture string
		switch {
		case star < 0:
			if spec != pattern {
				continue
			}
		case strings.HasPrefix(spec, pattern[:star]) &&
			strings.HasSuffix(spec, pattern[star+1:]) &&
			len(spec) >= len(pattern)-1:
			capture = spec[star : len(spec)-(len(pattern)-star-1)]
		default:
			continue
		}
		for _, target := range targets {
			out = append(out, strings.ReplaceAll(target, "*", capture))
		}
	}
	if len(out) == 0 && c.baseURL != "" {
		out = append(out, filepath.ToSlash(filepath.Clean(filepath.Join(c.baseURL, spec))))
	}
	return out
}

// stripJSONC removes // and /* */ comments and trailing commas outside strings,
// so tsconfig.json (which is JSONC) can be decoded by encoding/json.
func stripJSONC(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString, escaped := false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(src) {
			if src[i+1] == '/' {
				for i < len(src) && src[i] != '\n' {
					i++
				}
				if i < len(src) {
					out = append(out, '\n')
				}
				continue
			}
			if src[i+1] == '*' {
				i += 2
				for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		out = append(out, c)
	}
	// Strip trailing commas before } or ].
	cleaned := make([]byte, 0, len(out))
	inString, escaped = false, false
	for i := 0; i < len(out); i++ {
		c := out[i]
		if inString {
			cleaned = append(cleaned, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			cleaned = append(cleaned, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(out) && (out[j] == ' ' || out[j] == '\t' || out[j] == '\r' || out[j] == '\n') {
				j++
			}
			if j < len(out) && (out[j] == '}' || out[j] == ']') {
				continue
			}
		}
		cleaned = append(cleaned, c)
	}
	return cleaned
}
