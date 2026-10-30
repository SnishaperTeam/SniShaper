package app

// Profile lifecycle tests. Everything runs against temp directories: the
// user-config location is redirected through AppData/XDG_CONFIG_HOME, so the
// developer's real profiles are never touched.

import (
	"os"
	"path/filepath"
	"testing"

	corelog "github.com/SnishaperTeam/snishaper-core/log"
	"github.com/SnishaperTeam/snishaper-core/proxy"
)

// newProfileTestApp builds an App whose live config is a temp copy of the
// repository's bundled config, with the profile storage redirected to a
// temp user-config dir.
func newProfileTestApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	t.Setenv("AppData", root)        // Windows os.UserConfigDir
	t.Setenv("XDG_CONFIG_HOME", root) // Unix os.UserConfigDir

	liveSettings := filepath.Join(root, "live-settings.json")
	liveRules := filepath.Join(root, "live-rules.json")
	for _, pair := range []struct{ src, dst string }{
		{filepath.Join("..", "config", "settings.json"), liveSettings},
		{filepath.Join("..", "rules", "config.json"), liveRules},
	} {
		data, err := os.ReadFile(pair.src)
		if err != nil {
			t.Fatalf("read fixture %s: %v", pair.src, err)
		}
		if err := os.WriteFile(pair.dst, data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	rm := proxy.NewRuleManager(liveSettings, liveRules, proxy.WithLogger(corelog.Std))
	if err := rm.LoadConfig(); err != nil {
		t.Fatalf("load fixture config: %v", err)
	}
	return &App{
		ruleManager:  rm,
		proxyServer:  proxy.NewProxyServer("127.0.0.1:18080", proxy.WithLogger(corelog.Std)),
		settingsPath: liveSettings,
		rulesPath:    liveRules,
		logDir:       filepath.Join(root, "logs"),
	}
}

func TestProfileNameValidation(t *testing.T) {
	valid := []string{"work", "Home Network", "家中", "a", "profile_2", "x-y"}
	invalid := []string{"", "_reserved", "a/b", "..", ".hidden", "a:b", "a<b"}
	for _, name := range valid {
		if err := validateProfileName(name); err != nil {
			t.Errorf("name %q should be valid, got: %v", name, err)
		}
	}
	for _, name := range invalid {
		if err := validateProfileName(name); err == nil {
			t.Errorf("name %q should be rejected", name)
		}
	}
}

func TestProfileLifecycle(t *testing.T) {
	a := newProfileTestApp(t)

	// No profiles and no active profile to start with.
	if act := a.GetActiveProfile(); act != "" {
		t.Fatalf("active profile = %q, want empty", act)
	}
	if list := a.ListProfiles(); len(list) != 0 {
		t.Fatalf("expected no profiles, got %d", len(list))
	}

	// Create a profile from the current live config.
	if _, err := a.CreateProfile("work"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if list := a.ListProfiles(); len(list) != 1 || list[0].Name != "work" {
		t.Fatalf("list = %+v", list)
	}

	// Duplicate and rename.
	if err := a.DuplicateProfile("work", "home"); err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if err := a.RenameProfile("home", "home2"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := a.CreateProfile("work"); err == nil {
		t.Fatal("creating a duplicate name must fail")
	}

	// Switch: live files must end up equal to the profile snapshot.
	if err := a.SwitchProfile("work"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if act := a.GetActiveProfile(); act != "work" {
		t.Fatalf("active = %q, want work", act)
	}
	livePairs := []struct{ live, prof string }{
		{a.settingsPath, filepath.Join(profileDir("work"), "settings.json")},
		{a.rulesPath, filepath.Join(profileDir("work"), "rules.json")},
	}
	for _, pair := range livePairs {
		live, err := os.ReadFile(pair.live)
		if err != nil {
			t.Fatal(err)
		}
		prof, err := os.ReadFile(pair.prof)
		if err != nil {
			t.Fatal(err)
		}
		if string(live) != string(prof) {
			t.Errorf("live %s differs from profile after switch", pair.live)
		}
	}
	if err := a.SwitchProfile("work"); err == nil {
		t.Fatal("switching to the already-active profile must fail")
	}

	// Switch away and back: the outgoing profile keeps its identity and the
	// marker follows the switches.
	if err := a.SwitchProfile("home2"); err != nil {
		t.Fatalf("switch to home2: %v", err)
	}
	if act := a.GetActiveProfile(); act != "home2" {
		t.Fatalf("active = %q, want home2", act)
	}
	if err := a.SwitchProfile("work"); err != nil {
		t.Fatalf("switch back to work: %v", err)
	}

	// Export + import round-trip through a bundle.
	path, err := a.ExportProfile("work")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	bundle, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a bundle from another machine: rename inside the JSON is not
	// possible via the API, so import under the same name must fail as a
	// conflict, proving validation runs before writing.
	if _, err := a.ImportProfile(string(bundle)); err == nil {
		t.Fatal("importing over an existing profile must fail")
	}
}

func TestImportProfileRejectsInvalidBundle(t *testing.T) {
	a := newProfileTestApp(t)

	// Not JSON.
	if _, err := a.ImportProfile("{not json"); err == nil {
		t.Fatal("garbage must be rejected")
	}
	// Wrong format tag.
	if _, err := a.ImportProfile(`{"format":"something-else","version":1,"name":"x","settings":{},"rules":{}}`); err == nil {
		t.Fatal("wrong format must be rejected")
	}
	// Reserved name.
	if _, err := a.ImportProfile(`{"format":"snishaper-profile","version":1,"name":"_active","settings":{},"rules":{}}`); err == nil {
		t.Fatal("reserved name must be rejected")
	}
	// Missing rules payload.
	if _, err := a.ImportProfile(`{"format":"snishaper-profile","version":1,"name":"broken","settings":{"listen_port":8080},"rules":null}`); err == nil {
		t.Fatal("missing rules must be rejected")
	}
	// Settings that are not a JSON object fail the load pipeline's probe.
	if _, err := a.ImportProfile(`{"format":"snishaper-profile","version":1,"name":"broken","settings":[1,2,3],"rules":{"site_groups":[]}}`); err == nil {
		t.Fatal("non-object settings must be rejected by the load pipeline")
	}
	// Rules from a newer config version must be rejected (downgrade guard).
	futureRules := `{"config_version":99999,"site_groups":[]}`
	if _, err := a.ImportProfile(`{"format":"snishaper-profile","version":1,"name":"broken","settings":{},"rules":` + futureRules + `}`); err == nil {
		t.Fatal("future-version rules must be rejected by the load pipeline")
	}
}
