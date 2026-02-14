//go:build !darwin && !freebsd && !netbsd && !openbsd

package main

import "os"

func registerSIGINFO(_ chan<- os.Signal) {
	// SIGINFO is not available on this platform.
}
