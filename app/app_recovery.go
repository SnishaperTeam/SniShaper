package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"snishaper/pkg/sysproxy"
)

// LeftoverSystemProxyReport describes network state detected at startup that
// a crashed previous instance left behind, and what was done about it.
// Surfaced through the app:network_recovery event so the UI can show what
// was detected and changed.
type LeftoverSystemProxyReport struct {
	Detected       bool   `json:"detected"`
	Action         string `json:"action"` // "recovered" | "left" | "error" | "clean"
	Server         string `json:"server,omitempty"`
	ExpectedServer string `json:"expected_server,omitempty"`
	MarkerTime     string `json:"marker_time,omitempty"`
	Error          string `json:"error,omitempty"`
}

// decideLeftoverRecovery is the pure decision core of the startup recovery
// scan. It never touches system state, which keeps it unit-testable.
//
// markerData is the content of the managed-system-proxy marker written when
// system proxy was enabled; enabled/server describe the system proxy state
// observed at startup. Outcomes:
//
//	"none"    no marker, or a corrupt marker — nothing to do
//	"clean"   marker exists but the system proxy is off — just clear it
//	"recover" the system proxy still points at the server recorded in our
//	          marker, so it is verifiably ours and safe to disable
//	"leave"   the system proxy points somewhere else — the user or another
//	          app owns the setting now, never overwrite it
func decideLeftoverRecovery(markerData []byte, enabled bool, server string) (action, expectedServer, markerTime string) {
	var marker struct {
		ExpectedServer string `json:"expected_server"`
		Timestamp      string `json:"timestamp"`
	}
	if err := json.Unmarshal(markerData, &marker); err != nil {
		return "none", "", ""
	}
	if marker.ExpectedServer == "" {
		return "none", "", ""
	}
	if !enabled {
		return "clean", marker.ExpectedServer, marker.Timestamp
	}
	if strings.EqualFold(strings.TrimSpace(server), marker.ExpectedServer) {
		return "recover", marker.ExpectedServer, marker.Timestamp
	}
	return "left", marker.ExpectedServer, marker.Timestamp
}

// recoverLeftoverSystemProxy runs once at startup. A marker left behind by a
// crashed instance means the previous run enabled the system proxy and never
// disabled it. Only a system proxy that still points at the server recorded
// in our own marker is ever disabled — user or third-party settings are
// reported, never overwritten. Failures (e.g. missing admin rights) are
// reported with an actionable message and leave the system state untouched.
// The result is stored on the App for the lifetime of the process so the UI
// can query it via GetNetworkRecoveryReport once it has mounted.
func (a *App) recoverLeftoverSystemProxy() LeftoverSystemProxyReport {
	report := LeftoverSystemProxyReport{}

	markerData, err := os.ReadFile(a.proxyMarkerPath)
	if err != nil {
		// No marker: the previous run either never enabled the system proxy
		// or shut it down cleanly. Nothing to recover.
		return report
	}

	status := a.GetSystemProxyStatus()
	action, expected, markerTime := decideLeftoverRecovery(markerData, status.Enabled, status.Server)
	report.Detected = action != "none"
	report.Server = status.Server
	report.ExpectedServer = expected
	report.MarkerTime = markerTime

	switch action {
	case "none":
		report.Detected = false
		return report
	case "clean":
		// The system proxy is already off; just drop our stale marker.
		_ = a.clearManagedSystemProxyMarker()
		report.Action = "clean"
		a.appendLog("[recovery] stale managed-system-proxy marker cleared (system proxy already off)")
	case "left":
		// Someone else owns the system proxy now: report, drop our marker,
		// and never touch the setting.
		_ = a.clearManagedSystemProxyMarker()
		report.Action = "left"
		a.appendLog(fmt.Sprintf("[recovery] leftover system proxy marker found, but system proxy points at %s (expected %s); leaving user settings untouched",
			status.Server, expected))
	case "recover":
		report.Action = "recover"
		a.appendLog(fmt.Sprintf("[recovery] crashed run left the system proxy enabled at %s; disabling", status.Server))
		if err := sysproxy.DisableSystemProxy(); err != nil {
			report.Action = "error"
			report.Error = err.Error()
			a.appendLog("[recovery] failed to disable leftover system proxy: " + err.Error())
			return report
		}
		_ = a.clearManagedSystemProxyMarker()
		report.Action = "recovered"
		a.appendLog("[recovery] leftover system proxy disabled successfully")
	default:
		// unreachable: decideLeftoverRecovery only returns the cases above
		report.Detected = false
	}
	return report
}

// GetNetworkRecoveryReport returns the last startup recovery result so the
// UI can show what was detected after the fact (the report lives for the
// whole session; there is no event because recovery runs before the
// frontend mounts).
func (a *App) GetNetworkRecoveryReport() LeftoverSystemProxyReport {
	return a.recoveryReport
}
