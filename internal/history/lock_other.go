//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package history

// lockStore is a no-op on platforms without syscall.Flock. Record remains
// safe for single-process use and O_APPEND keeps appends atomic.
func lockStore(string) (func(), error) {
	return func() {}, nil
}
