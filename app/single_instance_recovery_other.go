//go:build !windows

package app

// RecoverBrokenSingleInstance recovers from a broken single instance state.
func RecoverBrokenSingleInstance(string) {
}

func AllowSingleInstanceCrossIntegrity(string) {
}

func IsSingleInstanceRunning(string) bool {
	return false
}

func WakeSingleInstance(string) error {
	return nil
}

func KillSingleInstance(string) {
}
