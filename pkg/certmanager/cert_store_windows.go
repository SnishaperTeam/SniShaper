//go:build windows

package certmanager

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Windows trust store: certificates live in the CurrentUser/LocalMachine
// Root and CA stores and are manipulated with certutil. Enumeration uses
// PowerShell because certutil output would have to be parsed by hand for the
// subject/expiry fields.

func platformInstallHelp() string {
	return "双击 CA 证书文件 -> 安装证书 -> 导入到\"受信任的根证书颁发机构\"（当前用户或本地计算机）"
}

func platformCACertInstalled(thumbprint string) bool {
	clean := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, ":", "")
		return s
	}
	cleanThumb := clean(thumbprint)

	// Run native certutil to check if the cert thumbprint exists in the User
	// Root or CA store. This avoids any PowerShell ExecutionPolicy restriction
	// issues and does not require temp files.
	outputRoot, _ := outputHiddenCommand("certutil", "-user", "-store", "root", thumbprint)
	if strings.Contains(clean(string(outputRoot)), cleanThumb) {
		return true
	}
	outputCA, _ := outputHiddenCommand("certutil", "-user", "-store", "ca", thumbprint)
	return strings.Contains(clean(string(outputCA)), cleanThumb)
}

func platformListCerts() ([]InstalledCert, error) {
	psScript := `
$stores = @(
  @{ Location = 'CurrentUser'; Name = 'Root' },
  @{ Location = 'CurrentUser'; Name = 'CA' },
  @{ Location = 'LocalMachine'; Name = 'Root' },
  @{ Location = 'LocalMachine'; Name = 'CA' }
)
$result = @()
foreach ($spec in $stores) {
  $store = New-Object System.Security.Cryptography.X509Certificates.X509Store($spec.Name, $spec.Location)
  try {
    $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadOnly)
    foreach ($cert in $store.Certificates) {
      if ($cert.Subject -match 'SniShaper' -or $cert.Issuer -match 'SniShaper') {
        $result += [PSCustomObject]@{
          subject = $cert.Subject
          thumbprint = $cert.Thumbprint
          notAfter = $cert.NotAfter.ToString('yyyy-MM-dd HH:mm:ss')
          storeName = $spec.Name
          storeLocation = $spec.Location
          token = "$($spec.Location)|$($spec.Name)|$($cert.Thumbprint)"
        }
      }
    }
  } finally {
    $store.Close()
  }
}
$result | ConvertTo-Json -Compress
`
	output, err := outputHiddenCommand("powershell", "-NoProfile", "-Command", psScript)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate certificate stores: %w", err)
	}

	text := strings.TrimSpace(string(output))
	if text == "" {
		return []InstalledCert{}, nil
	}

	var certs []InstalledCert
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal(output, &certs); err != nil {
			return nil, fmt.Errorf("failed to parse installed certificates: %w", err)
		}
		return certs, nil
	}

	var single InstalledCert
	if err := json.Unmarshal(output, &single); err != nil {
		return nil, fmt.Errorf("failed to parse installed certificate: %w", err)
	}
	return []InstalledCert{single}, nil
}

func platformInstallCA(caPath string) error {
	// Use certutil to install to the CurrentUser Root store. This pops up a
	// standard Windows security warning, so it runs visible (not hidden) to
	// keep that interactive dialog in front of the user.
	cmd := exec.Command("certutil", "-user", "-addstore", "root", caPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install CA certificate: %w", err)
	}
	fmt.Println("[Cert] CA certificate installed successfully to CurrentUser Root store")
	return nil
}

func platformUninstallCert(storeLocation, storeName, thumbprint string) error {
	if storeLocation == "" {
		storeLocation = "CurrentUser"
	}
	if storeName == "" {
		storeName = "Root"
	}

	args := []string{}
	if strings.EqualFold(storeLocation, "CurrentUser") {
		args = append(args, "-user")
	}
	args = append(args, "-delstore", storeName, thumbprint)

	if strings.EqualFold(storeLocation, "LocalMachine") {
		return runElevatedCommand("certutil", args...)
	}
	return runHiddenCommand("certutil", args...)
}

var sha1ThumbprintPattern = regexp.MustCompile(`(?i)[A-F0-9]{40}`)

func parseCertutilStoreOutput(output []byte, storeLocation, storeName string) []InstalledCert {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var blocks [][]string
	var current []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "====") {
			if len(current) > 0 {
				blocks = append(blocks, current)
			}
			current = []string{line}
			continue
		}
		if len(current) > 0 {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		blocks = append(blocks, current)
	}

	var certs []InstalledCert
	for _, block := range blocks {
		joined := strings.Join(block, "\n")
		if !strings.Contains(strings.ToLower(joined), "snishaper") {
			continue
		}

		var subject string
		var notAfter string
		var thumbprint string

		for _, line := range block {
			lower := strings.ToLower(line)
			if subject == "" && strings.Contains(lower, "snishaper") {
				if idx := strings.Index(line, ":"); idx >= 0 && idx+1 < len(line) {
					subject = strings.TrimSpace(line[idx+1:])
				}
			}
			if notAfter == "" && strings.Contains(lower, "notafter:") {
				if idx := strings.Index(line, ":"); idx >= 0 && idx+1 < len(line) {
					notAfter = strings.TrimSpace(line[idx+1:])
				}
			}
			if thumbprint == "" {
				if match := sha1ThumbprintPattern.FindString(line); match != "" {
					thumbprint = strings.ToUpper(match)
				}
			}
		}

		if thumbprint == "" {
			continue
		}
		if subject == "" {
			subject = "SniShaper CA"
		}

		certs = append(certs, InstalledCert{
			Subject:       subject,
			Thumbprint:    thumbprint,
			NotAfter:      notAfter,
			StoreName:     storeName,
			StoreLocation: storeLocation,
			Token:         storeLocation + "|" + storeName + "|" + thumbprint,
		})
	}

	return certs
}
