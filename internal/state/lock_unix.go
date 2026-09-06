//go:build !windows

package state

import (
	"os"

	"golang.org/x/sys/unix"
)

type fileLock struct {
	file *os.File
}

func acquireFileLock(lockPath string) (*fileLock, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &fileLock{file: f}, nil
}

func (l *fileLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	defer l.file.Close()
	return unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
}
