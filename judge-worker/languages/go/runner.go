package main

import (
	"os"
	"syscall"
)

func main() {
	arguments := append([]string{"/sandbox/artifact/main"}, os.Args[1:]...)
	if syscall.Exec(arguments[0], arguments, os.Environ()) != nil {
		os.Exit(127)
	}
}
