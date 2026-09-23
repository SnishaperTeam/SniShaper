//go:build !windows && !linux

package app

import (
	"fmt"
	"strings"
)

// installUpdateAsset has no implementation on this platform: the release has no
// bundle this app can unpack and replace itself with.
func (a *App) installUpdateAsset(localPath string) error {
	lower := strings.ToLower(localPath)
	switch {
	case strings.HasSuffix(lower, ".dmg"), strings.HasSuffix(lower, ".pkg"):
		return fmt.Errorf("this build cannot install %s automatically; open it manually", localPath)
	default:
		return fmt.Errorf("unsupported update file type on this platform: %s", localPath)
	}
}
