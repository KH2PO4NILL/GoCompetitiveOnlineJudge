package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

func main() {
	var mode string
	fmt.Scan(&mode)
	switch mode {
	case "re":
		os.Exit(7)
	case "tle":
		for {
		}
	case "mle":
		for {
			memory, err := syscall.Mmap(-1, 0, 1048576, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
			if err != nil {
				os.Exit(8)
			}
			for offset := 0; offset < len(memory); offset += 4096 {
				memory[offset] = 1
			}
		}
	case "ole":
		for {
			fmt.Print(strings.Repeat("x", 4096))
		}
	case "wa":
		fmt.Println(41)
	default:
		fmt.Println(42)
	}
}
