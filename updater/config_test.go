//go:build windows

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		current string
		target  string
		want    int
	}{
		{"1.30.0", "1.30.1", -1},
		{"1.30.1", "1.30.0", 1},
		{"1.30.0", "1.30.0", 0},
		{"1.30.0.0", "1.30.0", 0},
		{"1.30.0-beta.1", "1.30.0", -1},
		{"1.30.0", "1.30.0-rc.1", 1},
		{"1.30.0-beta.1", "1.30.0-beta.2", -1},
		{"1.30.0-beta.1", "1.30.0-rc.1", -1},
		{"v1.29.9", "1.30.0", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.current, c.target); got != c.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", c.current, c.target, got, c.want)
		}
	}
}

func TestManifestSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"version":"1.30.1"}`)
	sig := ed25519.Sign(priv, body)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	if err := verifyManifestSignature(body, sigB64, pubB64); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := verifyManifestSignature(body, string(sig), pubB64); err != nil {
		t.Fatalf("valid raw signature rejected: %v", err)
	}
	if err := verifyManifestSignature([]byte(`{"version":"1.30.2"}`), sigB64, pubB64); err == nil {
		t.Fatal("tampered manifest accepted")
	}
}

func TestReadInstalledMissingKey(t *testing.T) {
	info := readInstalled(Config{AppID: "{00000000-0000-0000-0000-000000000000}"})
	if info.Version != "" || info.Dir != "" {
		t.Fatalf("unexpected installed info: %+v", info)
	}
}

func TestResolvePackageURL(t *testing.T) {
	base := "https://example.com/releases/stable/manifest.json"
	if got := resolvePackageURL(base, "update.zip"); got != "https://example.com/releases/stable/update.zip" {
		t.Fatalf("relative url: %s", got)
	}
	if got := resolvePackageURL(base, "https://cdn.example.com/a.zip"); got != "https://cdn.example.com/a.zip" {
		t.Fatalf("absolute url: %s", got)
	}
}
