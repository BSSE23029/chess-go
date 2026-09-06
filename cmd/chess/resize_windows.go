//go:build windows

package main

import "os"

// Windows terminals do not expose SIGWINCH through os/signal. The renderer
// still uses the current terminal size whenever it redraws; resize events are
// simply observed on the next input or state change.
func watchTerminalResize(_ chan<- os.Signal) func() { return func() {} }
