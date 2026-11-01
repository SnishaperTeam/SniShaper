package app

// Network-state regression suite.
//
// Scope: the system proxy restoration state machine — the marker file, the
// startup recovery scan, and their behavior across repeated enable/disable
// cycles and failure paths. Everything here runs on any OS in CI without
// elevation: platform effects are simulated through the sysproxyStatusFn /
// sysproxyDisableFn seams, and only marker files in t.TempDir() are touched.
//
// TUN device cycles, live DNS resolution and route installation need
// elevated permissions and real hardware — see docs/regression-tests.md for
// what runs where and which cases stay manual.

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/SnishaperTeam/snishaper-core/proxy"

	"snishaper/pkg/sysproxy"
)

// withSimulatedSystemProxy swaps the platform seams for the duration of fn
// and restores them afterwards, so tests never touch the machine's real
// proxy settings and parallel tests cannot leak state into each other.
func withSimulatedSystemProxy(t *testing.T, enabled bool, server string, disableErr error, fn func()) {
	t.Helper()
	oldStatus, oldDisable := sysproxyStatusFn, sysproxyDisableFn
	sysproxyStatusFn = func() sysproxy.SystemProxyStatus {
		return sysproxy.SystemProxyStatus{Enabled: enabled, Server: server}
	}
	sysproxyDisableFn = func() error { return disableErr }
	defer func() {
		sysproxyStatusFn, sysproxyDisableFn = oldStatus, oldDisable
	}()
	fn()
}

func newRecoveryTestApp(t *testing.T) *App {
	t.Helper()
	return &App{proxyMarkerPath: filepath.Join(t.TempDir(), "managed-sysproxy.marker.json")}
}

func markerExists(t *testing.T, a *App) bool {
	t.Helper()
	_, err := os.Stat(a.proxyMarkerPath)
	return err == nil
}

// TestSystemProxyRestorationCycles drives the full lifecycle many times:
// enable (marker written) -> crash (recovery disables) -> clean shutdown
// (marker cleared while proxy off) -> re-enable on a new port. A regression
// in the marker bookkeeping (stale marker, wrong server, cleared marker
// after enable) breaks the cycle assertion.
func TestSystemProxyRestorationCycles(t *testing.T) {
	for cycle := 0; cycle < 20; cycle++ {
		a := newRecoveryTestApp(t)
		port := 8080 + cycle // port changes between runs on purpose

		// Enable: the previous run recorded its server in the marker.
		expected := "127.0.0.1:" + itoa(port)
		if err := a.saveManagedSystemProxyMarker(expected); err != nil {
			t.Fatalf("cycle %d: save marker: %v", cycle, err)
		}

		// Crash: system proxy still points at the recorded server; the
		// startup scan must disable it and clear the marker.
		withSimulatedSystemProxy(t, true, expected, nil, func() {
			report := a.recoverLeftoverSystemProxy()
			if !report.Detected || report.Action != "recovered" {
				t.Fatalf("cycle %d: recovery = %+v, want action=recovered", cycle, report)
			}
		})
		if markerExists(t, a) {
			t.Fatalf("cycle %d: marker still present after crash recovery", cycle)
		}

		// Re-enable on a new port, then shut down cleanly with the proxy
		// already off (e.g. OS restart wiped it): recovery must report the
		// stale marker as clean-up only and never call disable.
		next := "127.0.0.1:" + itoa(port+1)
		if err := a.saveManagedSystemProxyMarker(next); err != nil {
			t.Fatalf("cycle %d: save marker: %v", cycle, err)
		}
		withSimulatedSystemProxy(t, false, "", nil, func() {
			report := a.recoverLeftoverSystemProxy()
			if !report.Detected || report.Action != "clean" {
				t.Fatalf("cycle %d: recovery = %+v, want action=clean", cycle, report)
			}
		})
		if markerExists(t, a) {
			t.Fatalf("cycle %d: stale marker not cleared", cycle)
		}
	}
}

// TestRecoveryLeavesForeignProxyAlone: a leftover marker exists, but the
// system proxy now points somewhere else (user or another app owns it).
// Recovery must report and never disable.
func TestRecoveryLeavesForeignProxyAlone(t *testing.T) {
	a := newRecoveryTestApp(t)
	if err := a.saveManagedSystemProxyMarker("127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	withSimulatedSystemProxy(t, true, "10.0.0.5:3128", nil, func() {
		report := a.recoverLeftoverSystemProxy()
		if !report.Detected || report.Action != "left" {
			t.Fatalf("recovery = %+v, want action=left", report)
		}
	})
	if markerExists(t, a) {
		t.Fatal("marker must be dropped once another program owns the proxy")
	}
}

// TestRecoveryPermissionFailureRetriesNextStart: disabling fails (e.g. no
// admin rights). The error must surface and the marker must survive so the
// next startup retries the recovery.
func TestRecoveryPermissionFailureRetriesNextStart(t *testing.T) {
	a := newRecoveryTestApp(t)
	if err := a.saveManagedSystemProxyMarker("127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	permErr := &testError{"access is denied (simulated)"}
	withSimulatedSystemProxy(t, true, "127.0.0.1:8080", permErr, func() {
		report := a.recoverLeftoverSystemProxy()
		if !report.Detected || report.Action != "error" || report.Error == "" {
			t.Fatalf("recovery = %+v, want action=error with message", report)
		}
	})
	if !markerExists(t, a) {
		t.Fatal("marker must survive a failed recovery so the next start retries")
	}
}

// TestRecoveryConcurrentDecisions hammers the pure decision core from many
// goroutines (run with -race): concurrent startup paths reading the same
// marker bytes must not race or disagree.
func TestRecoveryConcurrentDecisions(t *testing.T) {
	marker := []byte(`{"expected_server":"127.0.0.1:8080","timestamp":"2026-01-01T00:00:00Z"}`)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			action, expected, _ := decideLeftoverRecovery(marker, true, "127.0.0.1:8080")
			if action != "recover" || expected != "127.0.0.1:8080" {
				t.Errorf("concurrent decision = %q/%q, want recover/127.0.0.1:8080", action, expected)
			}
		}()
	}
	wg.Wait()
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// withRecordedDisable swaps the disable seam for one that records whether it
// ran, and reports the flag after fn returns — so tests assert "the proxy
// was (not) touched" without touching the machine.
func withRecordedDisable(t *testing.T, fn func()) bool {
	t.Helper()
	oldDisable := sysproxyDisableFn
	disabled := false
	sysproxyDisableFn = func() error { disabled = true; return nil }
	defer func() { sysproxyDisableFn = oldDisable }()
	fn()
	return disabled
}

// newForceCleanupTestApp builds an App with a real (empty) RuleManager:
// ForceCleanup's ownership check goes through GetListenPort, which needs one.
func newForceCleanupTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.json")
	if err := os.WriteFile(rulesPath, []byte(`{"site_groups":[],"upstreams":[]}`), 0o644); err != nil {
		t.Fatalf("seed rules: %v", err)
	}
	rm := proxy.NewRuleManager(filepath.Join(dir, "settings.json"), rulesPath)
	if err := rm.LoadConfig(); err != nil {
		t.Fatalf("load rules: %v", err)
	}
	return &App{
		proxyMarkerPath: filepath.Join(dir, "managed-sysproxy.marker.json"),
		ruleManager:     rm,
	}
}

// TestForceCleanupLeavesForeignProxyAlone: the crash/force-exit path must
// not wipe a system proxy the app does not own (user's own or third-party).
func TestForceCleanupLeavesForeignProxyAlone(t *testing.T) {
	a := newForceCleanupTestApp(t)
	// No marker and a foreign server: not ours.
	var disabled bool
	// withRecordedDisable nests INSIDE withSimulatedSystemProxy: the status
	// helper also swaps the disable seam, so the recorder must win.
	withSimulatedSystemProxy(t, true, "10.0.0.5:3128", nil, func() {
		disabled = withRecordedDisable(t, func() { a.ForceCleanup() })
	})
	if disabled {
		t.Fatal("ForceCleanup must not disable a system proxy the app does not own")
	}
}

// TestForceCleanupDisablesOwnedProxy: when the live server is the app's own
// listen address, the crash path must still clean up.
func TestForceCleanupDisablesOwnedProxy(t *testing.T) {
	a := newForceCleanupTestApp(t)
	if err := a.saveManagedSystemProxyMarker("127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	var disabled bool
	withSimulatedSystemProxy(t, true, "127.0.0.1:8080", nil, func() {
		disabled = withRecordedDisable(t, func() { a.ForceCleanup() })
	})
	if !disabled {
		t.Fatal("ForceCleanup must disable the system proxy the app configured")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
