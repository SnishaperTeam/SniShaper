//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "SniShaper updater supports Windows only")
	os.Exit(1)
}
