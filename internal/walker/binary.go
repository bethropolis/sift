package walker

import (
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// knownTextExts are extensions that skip magic-number sniffing for speed.
var knownTextExts = map[string]bool{
	".go": true, ".rs": true, ".js": true, ".jsx": true, ".ts": true,
	".tsx": true, ".py": true, ".pyi": true, ".rb": true, ".php": true,
	".java": true, ".c": true, ".h": true, ".cpp": true, ".cc": true,
	".cxx": true, ".hpp": true, ".hh": true, ".cs": true, ".md": true,
	".txt": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".xml": true, ".html": true, ".htm": true, ".css": true, ".scss": true,
	".sql": true, ".sh": true, ".bash": true, ".zsh": true, ".lua": true,
	".swift": true, ".kt": true, ".kts": true, ".env": true, ".gitignore": true,
}

// knownBinaryExts are extensions that skip magic-number sniffing for speed.
var knownBinaryExts = map[string]bool{
	".iso": true, ".dmg": true, ".img": true, ".zip": true, ".tar": true,
	".gz": true, ".7z": true, ".rar": true, ".bz2": true, ".xz": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".elf": true, ".appimage": true, ".class": true, ".jar": true, ".o": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".ico": true, ".bmp": true, ".mp4": true, ".mkv": true, ".mov": true,
	".avi": true, ".mp3": true, ".wav": true, ".flac": true, ".pdf": true,
	".db": true, ".sqlite": true, ".sqlite3": true, ".pcap": true,
}

// IsBinaryFile checks whether path points to a binary file.
// 1. Fast-path: Known source/text extensions return false (0 disk I/O).
// 2. Fast-path: Known binary extensions return true (0 disk I/O).
// 3. Fallback: Magic-number sniffing via gabriel-vasile/mimetype.
func IsBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if knownTextExts[ext] {
			return false
		}
		if knownBinaryExts[ext] {
			return true
		}
	}

	// Sniff magic numbers for extensionless or unknown-extension files.
	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		return false
	}

	// In mimetype's hierarchy, all text formats descend from "text/plain".
	// Is("text/plain") walks the parent chain to verify if the file is text.
	return !mtype.Is("text/plain")
}
