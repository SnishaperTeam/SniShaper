//go:build linux && !headless

package app

import (
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	instanceListenerOnce sync.Once
	instanceListener     net.Listener
	instanceListenerPath string
	instanceUniqueID     string
)

// StartInstanceListener remembers which single instance channel to serve. The
// listener itself is opened once the service startup hook runs, so its messages
// end up in the app log instead of stderr.
func (a *App) StartInstanceListener(uniqueID string) {
	instanceUniqueID = uniqueID
}

// serveInstanceListener opens the wake-up channel a second launch uses to reach
// this process. It complements the runtime's session bus handling, which is
// unavailable when no D-Bus session bus can be reached.
func (a *App) serveInstanceListener() {
	if strings.TrimSpace(instanceUniqueID) == "" {
		return
	}
	uniqueID := instanceUniqueID

	instanceListenerOnce.Do(func() {
		path := instanceSocketPath(uniqueID)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("[single-instance] cannot clear %s: %v", path, err)
		}

		listener, err := net.Listen("unix", path)
		if err != nil {
			log.Printf("[single-instance] listen on %s failed: %v", path, err)
			return
		}
		if err := os.Chmod(path, 0600); err != nil {
			log.Printf("[single-instance] chmod %s failed: %v", path, err)
		}

		instanceListener = listener
		instanceListenerPath = path
		log.Printf("[single-instance] listening on %s", path)
		go a.serveInstanceWake(listener)
	})
}

func (a *App) serveInstanceWake(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			buf := make([]byte, 64)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			if !strings.HasPrefix(strings.TrimSpace(string(buf[:n])), instanceWakePayload) {
				return
			}
			log.Printf("[single-instance] wake requested by a second launch")
			a.RevealMainWindow()
		}()
	}
}

func stopInstanceListener() {
	instanceUniqueID = ""
	if instanceListener != nil {
		_ = instanceListener.Close()
		instanceListener = nil
	}
	if instanceListenerPath != "" {
		_ = os.Remove(instanceListenerPath)
		instanceListenerPath = ""
	}
}
