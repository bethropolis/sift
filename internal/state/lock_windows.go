//go:build windows

package state

import (
	"os"

	"golang.org/x/sys/windows"
)

type fileLock struct {
	file *os.File
}

func acquireFileLock(lockPath string) (*fileLock, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	var overlapped windows.Overlapped
	handle := windows.Handle(f.Fd())
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK)

	err = windows.LockFileEx(handle, flags, 0, 1, 0, &overlapped)
	if err != nil {
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

	var overlapped windows.Overlapped
	handle := windows.Handle(l.file.Fd())
	return windows.UnlockFileEx(handle, 0, 1, 0, &overlapped)
}
