package app

import (
	"encoding/json"
	"testing"
)

func markerJSON(t *testing.T, expected, timestamp string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"expected_server": expected,
		"timestamp":       timestamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecideLeftoverRecovery(t *testing.T) {
	cases := []struct {
		name     string
		marker   []byte
		enabled  bool
		server   string
		wantAct  string
		wantExp  string
	}{
		// Forced termination: marker present, system proxy still points at
		// the server we recorded — verifiably ours, safe to disable.
		{"crash leaves our proxy on", markerJSON(t, "127.0.0.1:8080", "2026-01-01T00:00:00Z"), true, "127.0.0.1:8080", "recover", "127.0.0.1:8080"},
		// Case/whitespace variants of the same server still count as ours.
		{"server case and space tolerant", markerJSON(t, "127.0.0.1:8080", "t"), true, " 127.0.0.1:8080 ", "recover", "127.0.0.1:8080"},
		// Interrupted shutdown left the marker, but the user (or another app)
		// has since taken over the system proxy — never overwrite it.
		{"user owns the proxy now", markerJSON(t, "127.0.0.1:8080", "t"), true, "10.0.0.5:3128", "left", "127.0.0.1:8080"},
		// Marker present but system proxy off (system restart wipes it on
		// some platforms, or the OS cleaned up): nothing to disable.
		{"marker but proxy off", markerJSON(t, "127.0.0.1:8080", "t"), false, "", "clean", "127.0.0.1:8080"},
		// No marker: previous run shut down cleanly or never enabled it.
		{"no marker", nil, true, "10.0.0.5:3128", "none", ""},
		// Corrupt marker (failed startup while writing it): never guess.
		{"corrupt marker", []byte("{not json"), true, "127.0.0.1:8080", "none", ""},
		// Marker without the expected server field is not actionable.
		{"marker missing field", []byte(`{"timestamp":"t"}`), true, "127.0.0.1:8080", "none", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			action, expected, _ := decideLeftoverRecovery(c.marker, c.enabled, c.server)
			if action != c.wantAct {
				t.Fatalf("action = %q, want %q", action, c.wantAct)
			}
			if expected != c.wantExp {
				t.Fatalf("expected server = %q, want %q", expected, c.wantExp)
			}
		})
	}
}
