package certmanager

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"snishaper/common"
)

type CertManager struct {
	caCert  *x509.Certificate
	caKey   *rsa.PrivateKey
	certMu  sync.RWMutex
	caPath  string
	keyPath string

	// Cache for CA install status to avoid expensive PowerShell calls
	lastStatus CAInstallStatus
	lastCheck  time.Time
}

func NewCertManager(caPath, keyPath string) *CertManager {
	return &CertManager{
		caPath:  caPath,
		keyPath: keyPath,
	}
}

func (cm *CertManager) generateCAUnlocked() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"SniShaper"},
			CommonName:   "SniShaper CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return err
	}

	cm.caCert = cert
	cm.caKey = privateKey

	if err := cm.saveCA(); err != nil {
		return err
	}

	fmt.Println("[Cert] CA certificate generated successfully")
	return nil
}

func (cm *CertManager) GenerateCA() error {
	cm.certMu.Lock()
	defer cm.certMu.Unlock()
	return cm.generateCAUnlocked()
}

func (cm *CertManager) saveCA() error {
	caFile, err := os.Create(cm.caPath)
	if err != nil {
		return err
	}
	defer caFile.Close()

	if err := pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: cm.caCert.Raw}); err != nil {
		return err
	}

	keyFile, err := os.OpenFile(cm.keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyFile.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(cm.caKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return err
	}

	return nil
}

func (cm *CertManager) LoadCA() error {
	cm.certMu.Lock()
	defer cm.certMu.Unlock()

	caData, err := os.ReadFile(cm.caPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cm.generateCAUnlocked()
		}
		return err
	}

	block, _ := pem.Decode(caData)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	keyData, err := os.ReadFile(cm.keyPath)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(keyData)
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	cm.caCert = cert
	cm.caKey = key

	return nil
}

func (cm *CertManager) GetCACertPath() string {
	return cm.caPath
}

func (cm *CertManager) GetCertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	if cm.caCert != nil {
		pool.AddCert(cm.caCert)
	}
	return pool
}

func (cm *CertManager) GetCA() *x509.Certificate {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	return cm.caCert
}

func (cm *CertManager) GetCACert() *x509.Certificate {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	return cm.caCert
}

func (cm *CertManager) GetCAKey() interface{} {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	return cm.caKey
}

func (cm *CertManager) IsCAInstalled() bool {
	status := cm.GetCAInstallStatus()
	return status.Installed
}

type CAInstallStatus struct {
	Installed   bool
	Platform    string
	CertPath    string
	InstallHelp string
}

func (cm *CertManager) GetCAInstallStatus() CAInstallStatus {
	cm.certMu.Lock()
	if !cm.lastCheck.IsZero() && time.Since(cm.lastCheck) < 5*time.Minute {
		status := cm.lastStatus
		cm.certMu.Unlock()
		return status
	}
	cm.certMu.Unlock()

	status := CAInstallStatus{
		CertPath:    cm.caPath,
		Platform:    runtime.GOOS,
		InstallHelp: platformInstallHelp(),
	}

	if cm.caCert == nil {
		if err := cm.LoadCA(); err != nil {
			return status
		}
	}
	if cm.caCert == nil {
		return status
	}

	status.Installed = platformCACertInstalled(certThumbprint(cm.caCert))

	// Update cache
	cm.certMu.Lock()
	cm.lastStatus = status
	cm.lastCheck = time.Now()
	cm.certMu.Unlock()

	return status
}

// certThumbprint returns the uppercase hex SHA-1 of the certificate DER. Every
// platform trust store addresses certificates by it.
func certThumbprint(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func (cm *CertManager) InstallCA() error {
	if cm.caCert == nil || cm.caKey == nil {
		if err := cm.LoadCA(); err != nil {
			return err
		}
	}
	if cm.caPath == "" {
		return fmt.Errorf("CA certificate path is empty")
	}

	// Drop earlier generations of the CA before installing the current one.
	if certs, err := cm.GetInstalledCertificates(); err == nil {
		for _, c := range certs {
			_ = cm.UninstallCertificate(c.Token)
		}
	}

	if err := platformInstallCA(cm.caPath); err != nil {
		return err
	}

	cm.invalidateInstallStatusCache()

	fmt.Println("[Cert] CA certificate installed successfully")
	return nil
}

type InstalledCert struct {
	Subject       string `json:"subject"`
	Thumbprint    string `json:"thumbprint"`
	NotAfter      string `json:"notAfter"`
	StoreName     string `json:"storeName"`
	StoreLocation string `json:"storeLocation"`
	Token         string `json:"token"`
}

// GetInstalledCertificates lists the certificates this app owns in the platform
// trust store.
func (cm *CertManager) GetInstalledCertificates() ([]InstalledCert, error) {
	return platformListCerts()
}

func (cm *CertManager) UninstallCertificate(thumbprint string) error {
	if thumbprint == "" {
		return fmt.Errorf("thumbprint is empty")
	}

	storeLocation := ""
	storeName := ""
	certThumbprint := thumbprint
	if parts := strings.SplitN(thumbprint, "|", 3); len(parts) == 3 {
		storeLocation = parts[0]
		storeName = parts[1]
		certThumbprint = parts[2]
	}

	if err := platformUninstallCert(storeLocation, storeName, certThumbprint); err != nil {
		return err
	}

	cm.invalidateInstallStatusCache()
	return nil
}

func (cm *CertManager) invalidateInstallStatusCache() {
	cm.certMu.Lock()
	cm.lastCheck = time.Time{}
	cm.certMu.Unlock()
}

func (cm *CertManager) OpenCertDir() error {
	return common.OpenTarget(filepath.Dir(cm.caPath))
}

func (cm *CertManager) OpenCAFile() error {
	return common.RevealPath(cm.caPath)
}

func (cm *CertManager) GetCACertPEM() string {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	if cm.caCert == nil {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.caCert.Raw}))
}

func (cm *CertManager) RegenerateCA() error {
	// 1. Try to clean up existing certificates from system store
	certs, err := cm.GetInstalledCertificates()
	if err == nil {
		for _, c := range certs {
			fmt.Printf("[Cert] Cleaning up old cert: %s\n", c.Thumbprint)
			_ = cm.UninstallCertificate(c.Token)
		}
	}

	// 2. Generate new CA
	cm.certMu.Lock()
	if err := cm.generateCAUnlocked(); err != nil {
		cm.certMu.Unlock()
		return err
	}
	cm.certMu.Unlock()

	fmt.Println("[Cert] CA certificate regenerated successfully")

	// 3. Install the new one
	return cm.InstallCA()
}

func (cm *CertManager) ExportCert() ([]byte, error) {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	if cm.caCert == nil {
		return nil, fmt.Errorf("no CA certificate available")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.caCert.Raw}), nil
}

func InitCertManager(certDir string) (*CertManager, error) {
	os.MkdirAll(certDir, 0755)

	cm := NewCertManager(
		filepath.Join(certDir, "ca.crt"),
		filepath.Join(certDir, "ca.key"),
	)

	if err := cm.LoadCA(); err != nil {
		_ = os.Remove(cm.caPath)
		_ = os.Remove(cm.keyPath)
		if genErr := cm.GenerateCA(); genErr != nil {
			return nil, fmt.Errorf("load existing CA failed: %v; regenerate failed: %w", err, genErr)
		}
	}

	return cm, nil
}
