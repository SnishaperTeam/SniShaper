//go:build !windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// tutorialMarkerPath returns the marker file used instead of the Windows
// registry on other platforms.
func tutorialMarkerPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "snishaper-tutorial-done")
	}
	return filepath.Join(base, "SniShaper", "tutorial_done")
}

// GetTutorialDone reports whether the first-launch tutorial has already been
// completed.
func (a *App) GetTutorialDone() bool {
	_, err := os.Stat(tutorialMarkerPath())
	return err == nil
}

// SetTutorialDone marks the tutorial as completed so it is not shown again on
// the next launch.
func (a *App) SetTutorialDone() error {
	path := tutorialMarkerPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create tutorial marker dir: %w", err)
	}
	if err := os.WriteFile(path, []byte("1"), 0o644); err != nil {
		return fmt.Errorf("write tutorial marker: %w", err)
	}
	return nil
}
