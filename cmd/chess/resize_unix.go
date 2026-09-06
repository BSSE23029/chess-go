//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func watchTerminalResize(resizes chan<- os.Signal) func() {
	signal.Notify(resizes, syscall.SIGWINCH)
	return func() { signal.Stop(resizes) }
}
