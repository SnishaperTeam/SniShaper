//go:build linux

package app

import "fmt"

// Single-instance guards are handled by wails v3 on Linux via a unix socket
// (SingleInstanceOptions in main.go); these are stubs kept for API
// compatibility with the auto-elevate path in main.go.
func IsSingleInstanceRunning(_ string) bool { return false }

func RecoverBrokenSingleInstance(_ string) {}

func WakeSingleInstance(_ string) error {
	return fmt.Errorf("not supported on linux")
}

func KillSingleInstance(_ string) {}

func AllowSingleInstanceCrossIntegrity(_ string) {}
