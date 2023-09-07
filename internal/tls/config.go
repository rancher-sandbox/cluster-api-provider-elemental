package tls

import (
	"fmt"

	"crypto/tls"
	"crypto/x509"

	"github.com/twpayne/go-vfs"
)

func GetCACert(fs vfs.FS, caCert string) ([]byte, error) {
	if _, err := fs.Stat(caCert); err == nil {
		return fs.ReadFile(caCert)
	}
	return []byte(caCert), nil
}

func GetTLSClientConfig(caCertPem []byte, useSystemCertPool bool, insecureSkipVerify bool) (*tls.Config, error) {
	var caCertPool *x509.CertPool
	var err error
	if useSystemCertPool {
		if caCertPool, err = x509.SystemCertPool(); err != nil {
			return nil, fmt.Errorf("copying system cert pool: %w", err)
		}
	} else {
		caCertPool = x509.NewCertPool()
	}
	if caCertPem != nil && len(caCertPem) > 0 {
		caCertPool.AppendCertsFromPEM(caCertPem)
	}
	return &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: insecureSkipVerify,
	}, nil
}
