//go:build linux

package app

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// wails implements single instance on Linux over the session bus, which is not
// present on every host (server installs, containers, minimal sessions).
// The app keeps its own unix socket as a fallback so a second launch can still
// wake the running instance instead of starting a second one.

const instanceWakePayload = "wake"

func IsSingleInstanceRunning(uniqueID string) bool {
	conn, err := net.DialTimeout("unix", instanceSocketPath(uniqueID), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// RecoverBrokenSingleInstance removes a socket left behind by a crashed run, so
// the next launch is not blocked by an unreachable endpoint.
func RecoverBrokenSingleInstance(uniqueID string) {
	path := instanceSocketPath(uniqueID)
	if _, err := os.Stat(path); err != nil {
		return
	}
	if IsSingleInstanceRunning(uniqueID) {
		return
	}
	if err := os.Remove(path); err == nil {
		log.Printf("[single-instance] removed stale socket %s", path)
	}
}

func WakeSingleInstance(uniqueID string) error {
	conn, err := net.DialTimeout("unix", instanceSocketPath(uniqueID), time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(instanceWakePayload + "\n")); err != nil {
		return err
	}
	return nil
}

// KillSingleInstance has no counterpart here: the socket is only held by a live
// instance, so there is nothing stale to force-terminate. A second launch that
// cannot wake the first one still falls back to the runtime's session bus path.
func KillSingleInstance(string) {}

// AllowSingleInstanceCrossIntegrity exists for the Windows integrity levels.
func AllowSingleInstanceCrossIntegrity(string) {}

func instanceSocketPath(uniqueID string) string {
	id := sanitizeInstanceID(uniqueID)
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return filepath.Join(dir, "snishaper-"+id+".sock")
		}
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("snishaper-%d-%s.sock", os.Getuid(), id))
}

func sanitizeInstanceID(uniqueID string) string {
	var b strings.Builder
	for _, r := range uniqueID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}
