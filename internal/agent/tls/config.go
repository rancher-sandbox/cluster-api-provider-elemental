package tls

import (
	"errors"
	"fmt"

	"crypto/tls"
	"crypto/x509"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/twpayne/go-vfs"
)

var ErrUnparsableCert = errors.New("could not parse certificate")

func GetCACert(fs vfs.FS, caCert string) ([]byte, error) {
	if _, err := fs.Stat(caCert); err == nil {
		bytes, err := fs.ReadFile(caCert)
		if err != nil {
			return nil, fmt.Errorf("reading CACert file: %w", err)
		}
		return bytes, nil
	}
	return []byte(caCert), nil
}

func GetTLSClientConfig(caCertPem []byte, useSystemCertPool bool, insecureSkipVerify bool) (*tls.Config, error) {
	var caCertPool *x509.CertPool
	var err error
	if useSystemCertPool {
		log.Debug("Using system cert pool")
		if caCertPool, err = x509.SystemCertPool(); err != nil {
			return nil, fmt.Errorf("copying system cert pool: %w", err)
		}
	} else {
		log.Debug("Using empty cert pool")
		caCertPool = x509.NewCertPool()
	}
	if len(caCertPem) > 0 {
		log.Debug("Adding caCert to pool")
		if ok := caCertPool.AppendCertsFromPEM(caCertPem); !ok {
			return nil, ErrUnparsableCert
		}
	}
	return &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec
	}, nil
}
