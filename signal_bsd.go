//go:build darwin || freebsd || netbsd || openbsd

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func registerSIGINFO(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGINFO)
}
