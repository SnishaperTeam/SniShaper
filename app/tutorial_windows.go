//go:build windows

package app

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const tutorialRegistryKey = `Software\SniShaper`

// GetTutorialDone reports whether the first-launch tutorial has already been
// completed. The flag lives in HKCU so it survives app updates and config
// resets.
func (a *App) GetTutorialDone() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, tutorialRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("tutorial_done")
	return err == nil && v == "1"
}

// SetTutorialDone marks the tutorial as completed so it is not shown again on
// the next launch.
func (a *App) SetTutorialDone() error {
	// CreateKey opens the key if it already exists.
	k, _, err := registry.CreateKey(registry.CURRENT_USER, tutorialRegistryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open tutorial registry key: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue("tutorial_done", "1"); err != nil {
		return fmt.Errorf("write tutorial_done: %w", err)
	}
	return nil
}
