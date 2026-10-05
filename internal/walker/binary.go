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
	".vue": true, ".svelte": true, ".astro": true, ".csv": true, ".log": true,
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
	".deb": true, ".rpm": true, ".msi": true, ".apk": true, ".ipa": true,
	".crx": true, ".whl": true, ".doc": true, ".docx": true, ".xls": true,
	".xlsx": true, ".ppt": true, ".pptx": true, ".epub": true, ".mobi": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".wasm": true,
	".zst": true, ".lz4": true, ".cab": true, ".pak": true, ".torrent": true,
	".vdi": true, ".vmdk": true, ".qcow2": true,
}

// IsBinaryFile checks whether path points to a binary file.
// 1. Fast-path: Known source/text extensions return false (0 disk I/O).
// 2. Fast-path: Known binary extensions return true (0 disk I/O).
// 3. Fallback: Magic-number sniffing via gabriel-vasile/mimetype.
func IsBinaryFile(path string) bool {
	if IsBinaryExt(path) {
		return true
	}
	if IsTextExt(path) {
		return false
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

// IsBinaryExt reports whether path has a known-binary extension, without any
// disk I/O. It backs the metadata walk so the skeleton never opens files.
func IsBinaryExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext != "" && knownBinaryExts[ext]
}

// IsTextExt reports whether path has a known-text extension, without any
// disk I/O.
func IsTextExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext != "" && knownTextExts[ext]
}

// binarySniffLen bounds the prefix sniffed for binary detection. The first
// 8KB holds any magic number while keeping detection O(1) per file instead
// of scanning multi-MB buffers byte-by-byte.
const binarySniffLen = 8192

// IsBinaryContent sniffs already-read bytes, avoiding the second open that
// DetectFile would cost after a read. Only a bounded prefix is examined, and
// a NUL byte short-circuits before mimetype runs. Callers must have applied
// the extension fast-paths first via IsBinaryExt/IsTextExt.
func IsBinaryContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	prefix := content
	if len(prefix) > binarySniffLen {
		prefix = prefix[:binarySniffLen]
	}
	for _, b := range prefix {
		if b == 0 {
			return true
		}
	}
	mtype := mimetype.Detect(prefix)
	return !mtype.Is("text/plain")
}
