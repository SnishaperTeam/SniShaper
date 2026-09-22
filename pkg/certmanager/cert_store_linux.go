//go:build linux

package certmanager

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Linux trust store: the CA goes into the distribution's anchor directory and is
// published with that distribution's update command. Chromium based browsers
// and command line tools read the resulting system bundle, so nothing else has
// to be written; Firefox keeps its own database and needs a manual import.

const caAnchorFileName = "snishaper-ca.crt"

var caAnchorDirs = []string{
	"/usr/local/share/ca-certificates",
	"/etc/pki/ca-trust/source/anchors",
	"/etc/ca-certificates/trust-source/anchors",
}

var caBundleFiles = []string{
	"/etc/ssl/certs/ca-certificates.crt",
	"/etc/pki/tls/certs/ca-bundle.crt",
	"/etc/ssl/ca-bundle.pem",
	"/var/lib/ca-certificates/ca-bundle.pem",
}

var caPublishedDirs = []string{
	"/etc/ssl/certs",
}

func platformInstallHelp() string {
	return "需要管理员权限：把 CA 复制到 /usr/local/share/ca-certificates/snishaper-ca.crt 后执行 update-ca-certificates" +
		"（Fedora 系为 /etc/pki/ca-trust/source/anchors + update-ca-trust，Arch 为 trust extract-compat）；" +
		"Firefox 使用独立证书库，需在设置中手动导入"
}

func platformCACertInstalled(thumbprint string) bool {
	certs, err := collectCACerts()
	if err != nil {
		return false
	}
	for _, cert := range certs {
		if strings.EqualFold(cert.Thumbprint, thumbprint) {
			return true
		}
	}
	return false
}

func platformListCerts() ([]InstalledCert, error) {
	certs, err := collectCACerts()
	if err != nil {
		return nil, err
	}

	result := []InstalledCert{}
	for _, cert := range certs {
		if !isSniShaperCert(cert.Cert) {
			continue
		}
		result = append(result, InstalledCert{
			Subject:       cert.Cert.Subject.String(),
			Thumbprint:    cert.Thumbprint,
			NotAfter:      cert.Cert.NotAfter.Format("2006-01-02 15:04:05"),
			StoreName:     cert.StoreName,
			StoreLocation: cert.StoreLocation,
			Token:         cert.StoreLocation + "|" + cert.StoreName + "|" + cert.Thumbprint,
		})
	}
	return result, nil
}

func platformInstallCA(caPath string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	dir := firstExistingAnchorDir()
	if dir == "" {
		return fmt.Errorf("no known CA anchor directory found (looked at %s)", strings.Join(caAnchorDirs, ", "))
	}

	data, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("read CA certificate: %w", err)
	}

	target := filepath.Join(dir, caAnchorFileName)
	if err := os.WriteFile(target, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	if err := publishAnchors(); err != nil {
		return err
	}

	fmt.Println("[Cert] CA certificate installed successfully to " + target)
	return nil
}

func platformUninstallCert(_, _, thumbprint string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	removed := 0
	for _, dir := range caAnchorDirs {
		matches, err := filepath.Glob(filepath.Join(dir, "snishaper*.crt"))
		if err != nil {
			continue
		}
		for _, path := range matches {
			if !anchorMatchesThumbprint(path, thumbprint) {
				continue
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove %s: %w", path, err)
			}
			removed++
		}
	}

	// A certificate that only exists in the published bundle (imported by hand
	// or left over from an older anchor name) is nothing this app owns, so
	// leaving it alone is the safe answer.
	if removed == 0 {
		fmt.Println("[Cert] no matching anchor file for " + thumbprint)
		return nil
	}
	if err := publishAnchors(); err != nil {
		return err
	}
	prunePublishedLinks()

	fmt.Printf("[Cert] removed %d anchor file(s) for %s\n", removed, thumbprint)
	return nil
}

type systemCert struct {
	Cert          *x509.Certificate
	Thumbprint    string
	StoreName     string
	StoreLocation string
}

// collectCACerts reads every certificate this platform can trust: the anchor
// files this app installs plus the published system bundles.
func collectCACerts() ([]systemCert, error) {
	var result []systemCert
	seen := map[string]bool{}

	add := func(cert *x509.Certificate, storeName, storeLocation string) {
		thumbprint := certThumbprint(cert)
		if seen[thumbprint] {
			return
		}
		seen[thumbprint] = true
		result = append(result, systemCert{
			Cert:          cert,
			Thumbprint:    thumbprint,
			StoreName:     storeName,
			StoreLocation: storeLocation,
		})
	}

	for _, dir := range caAnchorDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(strings.ToLower(entry.Name()), "snishaper") {
				continue
			}
			for _, cert := range parsePEMFile(filepath.Join(dir, entry.Name())) {
				add(cert, "Anchors", "System")
			}
		}
	}

	for _, bundle := range caBundleFiles {
		for _, cert := range parsePEMFile(bundle) {
			add(cert, "Bundle", "System")
		}
	}

	if len(result) == 0 {
		return nil, errors.New("no readable CA bundle or anchor directory found")
	}
	return result, nil
}

func parsePEMFile(path string) []*x509.Certificate {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		certs = append(certs, cert)
	}
	return certs
}

func isSniShaperCert(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	subject := strings.ToLower(cert.Subject.String())
	if strings.Contains(subject, "snishaper") {
		return true
	}
	return strings.Contains(strings.ToLower(cert.Issuer.String()), "snishaper")
}

// anchorMatchesThumbprint accepts an anchor file whose certificate carries the
// requested thumbprint. Unparsable files that still carry our own name are
// accepted too, so a corrupt anchor can always be cleaned up through the UI.
func anchorMatchesThumbprint(path, thumbprint string) bool {
	certs := parsePEMFile(path)
	if len(certs) == 0 {
		return true
	}
	for _, cert := range certs {
		if strings.EqualFold(certThumbprint(cert), thumbprint) {
			return true
		}
	}
	return false
}

func firstExistingAnchorDir() string {
	for _, dir := range caAnchorDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

// prunePublishedLinks removes links in the published certificate directory that
// point at an anchor this app just deleted, including the hash named ones the
// update command does not clean up on its own.
func prunePublishedLinks() {
	for _, dir := range caPublishedDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			link := filepath.Join(dir, entry.Name())
			target, err := os.Readlink(link)
			if err != nil {
				continue
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(dir, target)
			}
			if !insideAnchorDir(target) {
				continue
			}
			if _, err := os.Stat(target); err == nil {
				continue
			}
			if err := os.Remove(link); err != nil {
				fmt.Printf("[Cert] could not remove stale link %s: %v\n", link, err)
			}
		}
	}
}

func insideAnchorDir(path string) bool {
	clean := filepath.Clean(path)
	for _, dir := range caAnchorDirs {
		if strings.HasPrefix(clean, filepath.Clean(dir)+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// publishAnchors runs the update command of the current distribution so the
// freshly written anchor ends up in the system bundle.
func publishAnchors() error {
	type command struct {
		name string
		args []string
	}
	candidates := []command{
		{name: "update-ca-certificates", args: nil},
		{name: "update-ca-trust", args: []string{"extract"}},
		{name: "trust", args: []string{"extract-compat"}},
	}

	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate.name); err != nil {
			continue
		}
		out, err := runWithTimeout(60*time.Second, candidate.name, candidate.args...)
		if err != nil {
			return fmt.Errorf("%s failed: %w (%s)", candidate.name, err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	return fmt.Errorf("no CA update command found (tried %s, %s, trust); "+
		"the anchor file is in place, run the update manually and restart the browser",
		candidates[0].name, candidates[1].name)
}

func runWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return errors.New("installing the CA into the system trust store requires root; run SniShaper with sudo")
	}
	return nil
}
