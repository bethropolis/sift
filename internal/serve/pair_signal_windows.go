//go:build windows

package serve

import "os"

func pairSignalNotify(chan<- os.Signal) {}
func pairSignalStop(chan<- os.Signal)   {}
