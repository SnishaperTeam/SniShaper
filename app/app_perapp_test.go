package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SnishaperTeam/snishaper-core/proxy"
)

// seedPerAppRules writes a rules file with a deliberate conflict set between
// per-application, domain-only and default routing, mirroring the todo item
// "tests for conflicts between application, domain, IP, and default routing
// rules". The engine-level cases live in snishaper-core's process_rules_test;
// this test pins the app-facing surface (RuleManager.ExplainMatch — the exact
// call the Rules page explain tool makes through App.ExplainMatch).
func seedPerAppRules(t *testing.T) *proxy.RuleManager {
	t.Helper()
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "config.json")
	seed := `{"site_groups":[
		{"id":"g-proc","name":"proc","mode":"block","process":"telegram.exe,discord.exe","domains":[],"enabled":true},
		{"id":"g-proc-domain","name":"proc-domain","mode":"mitm","process":"chrome.exe","domains":["ads.example.com"],"enabled":true},
		{"id":"g-proc-only-elsewhere","name":"proc-other","mode":"block","process":"notepad.exe","domains":["example.com"],"enabled":true},
		{"id":"g-domain","name":"domain","mode":"direct","domains":["example.com"],"enabled":true}
	],"upstreams":[]}`
	if err := os.WriteFile(rulesPath, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed rules: %v", err)
	}
	rm := proxy.NewRuleManager(filepath.Join(dir, "settings.json"), rulesPath)
	if err := rm.LoadConfig(); err != nil {
		t.Fatalf("load rules: %v", err)
	}
	return rm
}

func TestPerAppRuleConflicts(t *testing.T) {
	rm := seedPerAppRules(t)

	cases := []struct {
		name       string
		host       string
		process    string
		wantSiteID string
		wantSource string
	}{
		// Application rule vs domain rule on the same host: process wins.
		{"process rule beats domain rule", "example.com", "telegram.exe", "g-proc", "manual"},
		// Unknown process ("" — lookup failed or unsupported platform):
		// process rules fail closed, the domain rule decides again.
		{"unknown process fails closed to domain rule", "example.com", "", "g-domain", "manual"},
		// Process+domain rule needs both to match; the domain restriction
		// narrows the process rule, not the reverse.
		{"process plus domain rule matches both", "ads.example.com", "chrome.exe", "g-proc-domain", "manual"},
		{"process plus domain rule misses on domain", "example.com", "chrome.exe", "g-domain", "manual"},
		{"process plus domain rule misses on process", "ads.example.com", "msedge.exe", "g-domain", "manual"},
		// No rule matches (process with no rules, unmatched host): default routing.
		{"default routing when nothing matches", "unmatched.example.org", "msedge.exe", "", "default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rm.ExplainMatch(tc.host, "mitm", tc.process)
			if !got.Simulated {
				t.Fatalf("ExplainMatch must be simulated")
			}
			if got.SiteID != tc.wantSiteID {
				t.Errorf("ExplainMatch(%q, %q).SiteID = %q, want %q", tc.host, tc.process, got.SiteID, tc.wantSiteID)
			}
			if got.Source != tc.wantSource {
				t.Errorf("ExplainMatch(%q, %q).Source = %q, want %q", tc.host, tc.process, got.Source, tc.wantSource)
			}
		})
	}
}
