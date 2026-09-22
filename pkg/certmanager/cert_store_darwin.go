//go:build darwin

package certmanager

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// macOS trust store: the CA is added to the login keychain with the security
// CLI, which asks the user for authorisation through the usual keychain prompt.
// The System keychain is only read, because writing to it needs root.

func platformInstallHelp() string {
	return "安装时会在钥匙串中请求授权；也可用\"钥匙串访问\"手动导入 CA 并把信任设置为\"始终信任\""
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
	keychain := loginKeychainPath()
	if keychain == "" {
		return errors.New("login keychain not found")
	}

	// -r trustRoot marks the certificate as a trusted root, which is what the
	// MITM proxy needs. macOS shows the keychain authorisation prompt here.
	out, err := outputHiddenCommand("security", "add-trusted-cert", "-r", "trustRoot", "-k", keychain, caPath)
	if err != nil {
		return fmt.Errorf("security add-trusted-cert failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	fmt.Println("[Cert] CA certificate installed successfully to " + keychain)
	return nil
}

func platformUninstallCert(storeLocation, _, thumbprint string) error {
	keychains := []string{loginKeychainPath()}
	if strings.EqualFold(storeLocation, "LocalMachine") {
		keychains = []string{systemKeychainPath()}
	}

	removed := false
	var lastErr error
	for _, keychain := range keychains {
		if keychain == "" {
			continue
		}
		out, err := outputHiddenCommand("security", "delete-certificate", "-Z", thumbprint, keychain)
		if err != nil {
			lastErr = fmt.Errorf("security delete-certificate failed: %w (%s)", err, strings.TrimSpace(string(out)))
			continue
		}
		removed = true
	}

	if !removed {
		if lastErr != nil {
			return lastErr
		}
		return errors.New("no keychain available for removal")
	}

	fmt.Println("[Cert] removed certificate " + thumbprint)
	return nil
}

type systemCert struct {
	Cert          *x509.Certificate
	Thumbprint    string
	StoreName     string
	StoreLocation string
}

func collectCACerts() ([]systemCert, error) {
	keychains := []struct {
		path          string
		storeName     string
		storeLocation string
	}{
		{path: loginKeychainPath(), storeName: "Login", storeLocation: "CurrentUser"},
		{path: systemKeychainPath(), storeName: "System", storeLocation: "LocalMachine"},
	}

	var result []systemCert
	seen := map[string]bool{}
	readable := false

	for _, keychain := range keychains {
		if keychain.path == "" {
			continue
		}
		out, err := outputHiddenCommand("security", "find-certificate", "-a", "-p", keychain.path)
		if err != nil {
			continue
		}
		readable = true

		rest := out
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
			thumbprint := certThumbprint(cert)
			if seen[thumbprint] {
				continue
			}
			seen[thumbprint] = true
			result = append(result, systemCert{
				Cert:          cert,
				Thumbprint:    thumbprint,
				StoreName:     keychain.storeName,
				StoreLocation: keychain.storeLocation,
			})
		}
	}

	if !readable {
		return nil, errors.New("certificate stores could not be read: the security command failed for every keychain")
	}
	return result, nil
}

func isSniShaperCert(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	if strings.Contains(strings.ToLower(cert.Subject.String()), "snishaper") {
		return true
	}
	return strings.Contains(strings.ToLower(cert.Issuer.String()), "snishaper")
}

func loginKeychainPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, name := range []string{"login.keychain-db", "login.keychain"} {
		path := filepath.Join(home, "Library", "Keychains", name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func systemKeychainPath() string {
	path := "/Library/Keychains/System.keychain"
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}
