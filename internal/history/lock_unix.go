//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package history

import (
	"os"
	"path/filepath"
	"syscall"
)

// lockStore takes an advisory exclusive lock on DataDir. History appends are
// already atomic with O_APPEND; the lock serializes pruning and Clear. A
// failed lock is best-effort and does not stop recording.
func lockStore(dir string) (func(), error) {
	path := filepath.Join(dir, LockFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return func() {}, err
	}
	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}
