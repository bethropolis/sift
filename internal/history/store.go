package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// HistoryFile is the JSONL log kept inside Store.DataDir.
	HistoryFile = "history.jsonl"
	// LockFile is the advisory lock used while appending and pruning.
	LockFile = "history.lock"
	// DefaultKeep is the retention used when Store.Keep is not positive.
	DefaultKeep = 20
)

// Store records and reads local dump history. DataDir holds history.jsonl;
// DumpsDir holds archived rendered documents. Keep is the number of entries
// retained after a record. Now supplies the clock for tests.
type Store struct {
	DataDir  string
	DumpsDir string
	Keep     int
	Now      func() time.Time
}

// HistoryPath returns the absolute JSONL history path.
func (s *Store) HistoryPath() string {
	return filepath.Join(s.DataDir, HistoryFile)
}

// Record archives rendered, appends a history entry, and prunes to Keep.
// Output records the original destination; File records the cache archive.
// Archive files are written with mode 0600 and directories with mode 0700.
func (s *Store) Record(root, cmd, style, output string, rendered []byte, tokens, files int) (Entry, error) {
	if s == nil {
		return Entry{}, errors.New("history: nil store")
	}
	if s.DataDir == "" || s.DumpsDir == "" {
		return Entry{}, errors.New("history: data and dumps directories are required")
	}
	if len(rendered) == 0 {
		return Entry{}, errors.New("history: empty render")
	}
	if err := ensureDir(s.DataDir); err != nil {
		return Entry{}, fmt.Errorf("history data directory: %w", err)
	}
	if err := ensureDir(s.DumpsDir); err != nil {
		return Entry{}, fmt.Errorf("history dumps directory: %w", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Entry{}, fmt.Errorf("history root: %w", err)
	}
	name := filepath.Base(absRoot)
	if filepath.Dir(absRoot) == absRoot {
		name = "/"
	}
	now := s.now()
	archive, err := writeArchive(s.DumpsDir, name, style, now, rendered)
	if err != nil {
		return Entry{}, fmt.Errorf("history archive: %w", err)
	}

	entry := Entry{
		V:      SchemaVersion,
		ID:     NewID(now),
		Ts:     now.UTC().Format(time.RFC3339),
		Cmd:    cmd,
		Root:   absRoot,
		Name:   name,
		File:   archive,
		Output: output,
		Bytes:  int64(len(rendered)),
		Tokens: tokens,
		Files:  files,
		Style:  style,
	}

	unlock, lockErr := lockStore(s.DataDir)
	if lockErr == nil {
		defer unlock()
	}
	if err := s.appendEntry(entry); err != nil {
		return Entry{}, fmt.Errorf("history append: %w", err)
	}
	if err := s.pruneLocked(); err != nil {
		return entry, fmt.Errorf("history prune: %w", err)
	}
	return entry, nil
}

// List returns entries newest first, limited to limit when limit is positive.
// Entries whose archive is missing, outside DumpsDir, or not a regular file
// are omitted.
func (s *Store) List(limit int) ([]Entry, error) {
	if s == nil {
		return nil, errors.New("history: nil store")
	}
	records, err := readRecords(s.HistoryPath())
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(records))
	for _, record := range records {
		if !record.ok {
			continue
		}
		if !entryFileExists(s.DumpsDir, record.entry.File) {
			continue
		}
		entries = append(entries, record.entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		ti, tj := entryTime(entries[i]), entryTime(entries[j])
		if ti.Equal(tj) {
			return entries[i].ID > entries[j].ID
		}
		return ti.After(tj)
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// Clear removes the history log and the archive files it references. Only
// files proven to be inside DumpsDir are deleted; unrelated files are never
// touched.
func (s *Store) Clear() error {
	if s == nil {
		return errors.New("history: nil store")
	}
	if s.DataDir == "" {
		return errors.New("history: data directory is required")
	}
	unlock, err := lockStore(s.DataDir)
	if err == nil {
		defer unlock()
	}
	records, err := readRecords(s.HistoryPath())
	if err != nil {
		return err
	}
	var errs []error
	seen := map[string]bool{}
	for _, record := range records {
		if !record.ok {
			continue
		}
		file := record.entry.File
		if file == "" || seen[file] {
			continue
		}
		seen[file] = true
		if !insideDir(s.DumpsDir, file) {
			continue
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove archive %q: %w", file, err))
		}
	}
	if err := os.Remove(s.HistoryPath()); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("remove history log: %w", err))
	}
	return errors.Join(errs...)
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) keep() int {
	if s.Keep > 0 {
		return s.Keep
	}
	return DefaultKeep
}

func (s *Store) appendEntry(entry Entry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	path := s.HistoryPath()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	n, writeErr := file.Write(line)
	if writeErr == nil && n != len(line) {
		writeErr = io.ErrShortWrite
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	return writeErr
}

func (s *Store) pruneLocked() error {
	records, err := readRecords(s.HistoryPath())
	if err != nil {
		return err
	}
	type candidate struct {
		index int
		entry Entry
		when  time.Time
	}
	candidates := make([]candidate, 0, len(records))
	for i, record := range records {
		if !record.ok {
			continue
		}
		candidates = append(candidates, candidate{index: i, entry: record.entry, when: entryTime(record.entry)})
	}
	keep := s.keep()
	if len(candidates) <= keep {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].when.Equal(candidates[j].when) {
			return candidates[i].entry.ID > candidates[j].entry.ID
		}
		return candidates[i].when.After(candidates[j].when)
	})

	keepIndex := make(map[int]bool, len(records))
	keptFiles := make(map[string]bool)
	for i, record := range records {
		if !record.ok {
			// Preserve lines we do not understand rather than destroying
			// another version's data.
			keepIndex[i] = true
		}
	}
	for i := 0; i < keep; i++ {
		keepIndex[candidates[i].index] = true
		keptFiles[candidates[i].entry.File] = true
	}

	lines := make([][]byte, 0, len(keepIndex))
	for i, record := range records {
		if keepIndex[i] {
			lines = append(lines, record.raw)
		}
	}
	if err := writeLinesAtomic(s.HistoryPath(), lines); err != nil {
		return err
	}

	var errs []error
	for _, dropped := range candidates[keep:] {
		file := dropped.entry.File
		if file == "" || keptFiles[file] || !insideDir(s.DumpsDir, file) {
			continue
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove pruned archive %q: %w", file, err))
		}
	}
	return errors.Join(errs...)
}

type record struct {
	raw   []byte
	entry Entry
	ok    bool
}

func readRecords(path string) ([]record, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read history: %w", err)
	}
	defer file.Close()

	records := make([]record, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}
		line := append([]byte(nil), raw...)
		var entry Entry
		ok := json.Unmarshal(line, &entry) == nil && entry.V == SchemaVersion
		records = append(records, record{raw: line, entry: entry, ok: ok})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan history: %w", err)
	}
	return records, nil
}

func writeLinesAtomic(path string, lines [][]byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".history-*.tmp")
	if err != nil {
		return fmt.Errorf("create history temp: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	writer := bufio.NewWriterSize(temp, 64*1024)
	for _, line := range lines {
		if _, err := writer.Write(line); err != nil {
			temp.Close()
			return fmt.Errorf("write history temp: %w", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			temp.Close()
			return fmt.Errorf("write history temp: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		temp.Close()
		return fmt.Errorf("flush history temp: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync history temp: %w", err)
	}
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("chmod history temp: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close history temp: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace history: %w", err)
	}
	return nil
}

func writeArchive(dir, name, style string, t time.Time, data []byte) (string, error) {
	path := archivePath(dir, name, t, style)
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for attempt := 1; attempt <= 100; attempt++ {
		if attempt > 1 {
			path = fmt.Sprintf("%s-%d%s", stem, attempt, ext)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		if err := file.Chmod(0o600); err != nil {
			file.Close()
			os.Remove(path)
			return "", err
		}
		n, writeErr := file.Write(data)
		if writeErr == nil && n != len(data) {
			writeErr = io.ErrShortWrite
		}
		if closeErr := file.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			os.Remove(path)
			return "", writeErr
		}
		return path, nil
	}
	return "", fmt.Errorf("no free archive name for %q", filepath.Base(path))
}

func ensureDir(path string) error {
	if path == "" {
		return errors.New("empty directory")
	}
	return os.MkdirAll(path, 0o700)
}

func entryFileExists(dumpsDir, file string) bool {
	if file == "" || !insideDir(dumpsDir, file) {
		return false
	}
	info, err := os.Stat(file)
	return err == nil && info.Mode().IsRegular()
}

func entryTime(entry Entry) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, entry.Ts); err == nil {
			return t
		}
	}
	if len(entry.ID) > 0 {
		if t, err := time.Parse("20060102T150405Z", strings.SplitN(entry.ID, "-", 2)[0]); err == nil {
			return t
		}
	}
	return time.Time{}
}
