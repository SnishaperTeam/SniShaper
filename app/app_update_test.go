package app

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func releaseAssetsForTest() []githubAsset {
	names := []string{
		"dongle.Snishaper_1.31.0.0_x64_x86_arm64.zip",
		"Snishaper-1.31.0-arm64Setup.exe",
		"Snishaper-1.31.0-x64Setup.exe",
		"Snishaper-1.31.0-x86Setup.exe",
		"Snishaper-1.31.0-amd64.AppImage",
		"Snishaper-1.31.0-arm64.AppImage",
		"snishaper-1.31.0_amd64.rpm",
		"snishaper-cli-darwin-amd64.zip",
		"snishaper-cli-darwin-arm64.zip",
		"snishaper-cli-linux-amd64.tar.gz",
		"snishaper-cli-linux-arm64.tar.gz",
		"snishaper-cli-windows-386.zip",
		"snishaper-cli-windows-amd64.zip",
		"snishaper-cli-windows-arm64.zip",
		"snishaper-linux-amd64.tar.gz",
		"snishaper-linux-arm64.tar.gz",
		"snishaper-linux-x64.tar.gz",
		"snishaper-windows-386.zip",
		"snishaper-windows-amd64.zip",
		"snishaper-windows-arm64.zip",
		"snishaper_1.31.0_amd64.deb",
		"snishaper_1.31.0_arm64.deb",
		"unsigned_dongle.Snishaper_1.31.0.0_x64_x86_arm64.msixbundle",
		"SniShaper-macos-arm64.zip",
		"SniShaper-macos-x86_64.zip",
		// Assets published before the switch to zip: they must still be
		// offered to installs that predate it.
		"dongle.Snishaper_1.29.1.0_x64_x86_arm64.7z",
		"snishaper-cli-windows-amd64.7z",
		"snishaper-windows-amd64.7z",
	}
	assets := make([]githubAsset, 0, len(names))
	for _, name := range names {
		assets = append(assets, githubAsset{Name: name, Size: 1, DownloadURL: "https://example.invalid/" + name})
	}
	return assets
}

func TestFilterUpdateAssetsKeepsCurrentTargetOnly(t *testing.T) {
	got := filterUpdateAssets(releaseAssetsForTest())
	if len(got) == 0 {
		t.Fatalf("no asset returned for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	wantOS := runtime.GOOS
	wantArch := runtimeAssetArch()
	for _, asset := range got {
		lower := strings.ToLower(asset.Name)
		if strings.HasPrefix(lower, "dongle.") || strings.Contains(lower, "unsigned") {
			t.Errorf("bundle/unsigned asset leaked into update list: %s", asset.Name)
		}
		if isCLIAsset(lower) != headlessBuild {
			t.Errorf("asset from the other flavour leaked into update list: %s", asset.Name)
		}
		if osName := assetOSToken(lower); osName != wantOS {
			t.Errorf("asset for %q leaked into %q update list: %s", osName, wantOS, asset.Name)
		}
		if arch := assetArchToken(lower); arch != wantArch {
			t.Errorf("asset for %q leaked into %q update list: %s", arch, wantArch, asset.Name)
		}
	}
}

func TestFilterUpdateAssetsWindowsAmd64Shortlist(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" || headlessBuild {
		t.Skip("shortlist assertion is for the desktop windows/amd64 build")
	}
	got := filterUpdateAssets(releaseAssetsForTest())
	// Installer first, then the zip every Windows 10 can open, then the
	// legacy 7z kept for old releases.
	want := []string{
		"Snishaper-1.31.0-x64Setup.exe",
		"snishaper-windows-amd64.zip",
		"snishaper-windows-amd64.7z",
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected asset count: got %d (%v), want %d (%v)", len(got), assetNames(got), len(want), want)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("asset %d = %s, want %s", i, got[i].Name, name)
		}
	}
	wantKinds := []string{"exe", "zip", "7z"}
	for i, kind := range wantKinds {
		if got[i].Kind != kind {
			t.Errorf("asset %d kind = %s, want %s", i, got[i].Kind, kind)
		}
	}
}

func TestFilterUpdateAssetsLegacyReleaseStillOffers7z(t *testing.T) {
	if runtime.GOOS != "windows" || headlessBuild {
		t.Skip("legacy archive assertion is for the desktop windows build")
	}
	legacy := []githubAsset{
		{Name: "Snishaper-1.29.1-x64Setup.exe", Size: 1, DownloadURL: "https://example.invalid/installer.exe"},
		{Name: "snishaper-windows-amd64.7z", Size: 1, DownloadURL: "https://example.invalid/portable.7z"},
		{Name: "snishaper-windows-amd64.zip", Size: 1, DownloadURL: "https://example.invalid/portable.zip"},
	}
	got := filterUpdateAssets(legacy)
	if len(got) < 2 {
		t.Fatalf("expected both archives to be offered, got %v", assetNames(got))
	}
	var kinds []string
	for _, asset := range got {
		kinds = append(kinds, asset.Kind)
	}
	if kinds[len(kinds)-1] != "7z" {
		t.Fatalf("legacy 7z must stay selectable last, got %v", kinds)
	}
}

func TestAssetArchToken(t *testing.T) {
	cases := map[string]string{
		"snishaper-windows-amd64.zip":           "x64",
		"snishaper-windows-386.zip":             "x86",
		"snishaper-windows-arm64.zip":           "arm64",
		"snishaper-windows-amd64.7z":            "x64",
		"Snishaper-1.29.1-x86Setup.exe":         "x86",
		"Snishaper-1.29.1-x64Setup.exe":         "x64",
		"snishaper-linux-x64.tar.gz":            "x64",
		"snishaper-cli-linux-amd64.tar.gz":      "x64",
		"SniShaper-macos-x86_64.zip":            "x64",
		"snishaper-cli-darwin-arm64.zip":        "arm64",
		"Snishaper-1.30.0-alpha.1-x64Setup.exe": "x64",
	}
	for name, want := range cases {
		if got := assetArchToken(strings.ToLower(name)); got != want {
			t.Errorf("assetArchToken(%q) = %q, want %q", name, got, want)
		}
	}
}

func assetNames(assets []ReleaseAsset) []string {
	names := make([]string, len(assets))
	for i, asset := range assets {
		names[i] = asset.Name
	}
	return names
}

func TestFilterUpdateAssetsExcludesUpdaterArtifacts(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" || headlessBuild {
		t.Skip("updater artefact assertion is for the desktop windows/amd64 build")
	}
	assets := []githubAsset{
		{Name: "Snishaper-1.31.0-x64Setup.exe", Size: 1, DownloadURL: "https://example.invalid/setup.exe"},
		{Name: "SniShaperUpdater-1.31.0-x64.exe", Size: 1, DownloadURL: "https://example.invalid/updater.exe"},
		{Name: "snishaper-1.31.0-windows-x64-update.zip", Size: 1, DownloadURL: "https://example.invalid/bundle.zip"},
		{Name: "updater-manifest-x64.json", Size: 1, DownloadURL: "https://example.invalid/manifest.json"},
		{Name: "snishaper-windows-amd64.zip", Size: 1, DownloadURL: "https://example.invalid/portable.zip"},
	}
	got := filterUpdateAssets(assets)
	for _, asset := range got {
		lower := strings.ToLower(asset.Name)
		if strings.HasPrefix(lower, "snishaperupdater-") || strings.HasSuffix(lower, "-update.zip") {
			t.Errorf("updater artefact leaked into update list: %s", asset.Name)
		}
	}
	want := []string{"Snishaper-1.31.0-x64Setup.exe", "snishaper-windows-amd64.zip"}
	if len(got) != len(want) {
		t.Fatalf("unexpected asset count: got %v, want %v", assetNames(got), want)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("asset %d = %s, want %s", i, got[i].Name, name)
		}
	}
}

// 回归 fixture：<title> 是发布名（可与 tag 不同），tag 必须取自 <link href>。
// 曾因把 title 当 tag 解析，"SniShaper v1" 被解析成 0，旧发布反超新发布。
const releasesAtomFixture = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry><title>SniShaper v0.4.1-rc.2</title><link rel="alternate" href="https://github.com/SnishaperTeam/SniShaper/releases/tag/v0.4.1-rc.2"/></entry>
  <entry><title>SniShaper v0.4.0</title><link rel="alternate" href="https://github.com/SnishaperTeam/SniShaper/releases/tag/v0.4.0"/></entry>
  <entry><title>v0.3.9-beta.4</title><link rel="alternate" href="https://github.com/SnishaperTeam/SniShaper/releases/tag/v0.3.9-beta.4"/></entry>
</feed>`

func TestParseReleasesFeed(t *testing.T) {
	tags, err := parseReleasesFeed(strings.NewReader(releasesAtomFixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"v0.4.1-rc.2", "v0.4.0", "v0.3.9-beta.4"}
	if len(tags) != len(want) {
		t.Fatalf("got %v, want %v", tags, want)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Fatalf("got %v, want %v", tags, want)
		}
	}
	if _, err := parseReleasesFeed(strings.NewReader("<feed></feed>")); err == nil {
		t.Fatal("feed without entries must be an error")
	}
}

func TestAtomReleasesChannelResolution(t *testing.T) {
	tags := []string{"v0.4.1-rc.2", "v0.4.0", "v0.3.9-beta.4"}
	releases := atomReleases(tags)
	if !releases[0].Prerelease || releases[1].Prerelease {
		t.Fatalf("prerelease flags must follow the tag channel: %+v", releases)
	}
	if rel := resolveChannelRelease(releases, "stable"); rel == nil || rel.TagName != "v0.4.0" {
		t.Fatalf("stable channel must resolve v0.4.0, got %+v", rel)
	}
	if rel := resolveChannelRelease(releases, "rc"); rel == nil || rel.TagName != "v0.4.1-rc.2" {
		t.Fatalf("rc channel must resolve v0.4.1-rc.2, got %+v", rel)
	}
}

func TestFetchReleasesFromMirrorsFirstSuccess(t *testing.T) {
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer fail.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"v1.30.2","assets":[]}]`))
	}))
	defer good.Close()
	apiURL := "https://api.github.com/repos/SnishaperTeam/SniShaper/releases?per_page=100"

	got := fetchReleasesFromMirrors([]string{fail.URL + "/", good.URL + "/"}, apiURL)
	if len(got) != 1 || got[0].TagName != "v1.30.2" {
		t.Fatalf("first successful mirror must win, got %+v", got)
	}
	if got := fetchReleasesFromMirrors([]string{fail.URL + "/"}, apiURL); got != nil {
		t.Fatalf("all mirrors failed: want nil, got %+v", got)
	}
}
