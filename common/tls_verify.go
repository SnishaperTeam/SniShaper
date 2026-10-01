package common

import (
	"crypto/x509"
	"errors"
	"time"
)

// VerifyChainOnly 校验对端证书链是否可由系统根信任，不匹配主机名。
// 用于 SNI 可能被主动伪造的场景：只要求对方持有一张可信 CA 签发的证书。
func VerifyChainOnly(peerCerts []*x509.Certificate) error {
	if len(peerCerts) == 0 {
		return errors.New("no peer certificates")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	intermediates := x509.NewCertPool()
	for _, cert := range peerCerts[1:] {
		intermediates.AddCert(cert)
	}
	_, err = peerCerts[0].Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   time.Now(),
	})
	return err
}
