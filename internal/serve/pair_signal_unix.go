//go:build !windows

package serve

import (
	"os"
	"os/signal"
	"syscall"
)

func pairSignalNotify(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGUSR1)
}

func pairSignalStop(ch chan<- os.Signal) {
	signal.Stop(ch)
}
