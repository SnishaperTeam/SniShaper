//go:build headless

package main

import (
	"os"

	"snishaper/internal/cliapp"
)

// The standalone CLI binary is now a thin shell around the shared cliapp
// dispatcher, which also backs the desktop binary's bundled CLI mode.
func main() {
	os.Exit(cliapp.Run(os.Args[1:], false))
}
