package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

func selfSignedCert(t *testing.T, commonName string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

// startSelfSignedTLSServer 起一个使用自签证书的本地 TLS 服务器
func startSelfSignedTLSServer(t *testing.T) string {
	t.Helper()
	cert, key := selfSignedCert(t, "127.0.0.1")
	tlsCert := tls.Certificate{Certificate: [][]byte{cert.Raw}, PrivateKey: key}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"http/1.1"},
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				tlsConn := tls.Server(c, cfg)
				_ = tlsConn.Handshake()
				time.Sleep(50 * time.Millisecond)
				_ = tlsConn.Close()
			}()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

func handshakeWithServer(t *testing.T, p *ProxyServer, addr, verifyName string) error {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	rule := Rule{Mode: "mitm"}
	uconn := p.GetUConn(conn, "example.com", verifyName, rule, true, "http/1.1", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return uconn.HandshakeContext(ctx)
}

func TestGetUConnNonECHDefaultsToChainOnly(t *testing.T) {
	addr := startSelfSignedTLSServer(t)
	p := NewProxyServer("127.0.0.1:0")
	// allowInsecure=true 但未配置 cert_verify：默认应回落到链校验，
	// 自签上游握手必须失败
	if err := handshakeWithServer(t, p, addr, "example.com"); err == nil {
		t.Fatal("expected handshake against self-signed upstream to fail with chain-only verification")
	}
}

func TestGetUConnBypassStillAcceptsSelfSigned(t *testing.T) {
	addr := startSelfSignedTLSServer(t)
	p := NewProxyServer("127.0.0.1:0")
	p.certBypassMap.Store(normalizeHost("example.com"), true)
	if err := handshakeWithServer(t, p, addr, "example.com"); err != nil {
		t.Fatalf("expected bypassed host to accept self-signed upstream, got: %v", err)
	}
}

func TestChainOnlyRejectsSelfSigned(t *testing.T) {
	cert, _ := selfSignedCert(t, "self-signed.example")
	verify := buildVerifyConnection("example.com", CertVerifyConfig{Mode: "chain_only"})
	err := verify(utls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}})
	if err == nil {
		t.Fatal("chain_only verification accepted a self-signed certificate")
	}
}

func TestChainOnlyAllowUnknownAuthorityAcceptsSelfSigned(t *testing.T) {
	cert, _ := selfSignedCert(t, "self-signed.example")
	verify := buildVerifyConnection("example.com", CertVerifyConfig{
		Mode:                  "chain_only",
		AllowUnknownAuthority: true,
	})
	if err := verify(utls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}); err != nil {
		t.Fatalf("expected self-signed certificate to be accepted with allow_unknown_authority, got: %v", err)
	}
}
