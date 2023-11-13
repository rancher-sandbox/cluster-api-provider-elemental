package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrPEMDecoding = errors.New("no PEM data found")
)

var _ Identity = (*Ed25519Identity)(nil)

type Ed25519Identity struct {
	privateKey ed25519.PrivateKey
}

func NewED25519Identity() (Identity, error) {
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating new key: %w", err)
	}
	return &Ed25519Identity{privateKey: privKey}, nil
}

func (i *Ed25519Identity) MarshalPublic() ([]byte, error) {
	x509key, err := x509.MarshalPKIXPublicKey(i.privateKey.Public())
	if err != nil {
		return nil, fmt.Errorf("marshalling public key: %w", err)
	}
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: x509key,
	})
	return keyPem, nil
}

func (i *Ed25519Identity) Sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(i.privateKey)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

func (i *Ed25519Identity) Marshal() ([]byte, error) {
	x509Key, err := x509.MarshalPKCS8PrivateKey(i.privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshalling key: %w", err)
	}
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509Key},
	)
	return keyPem, nil
}

func (i *Ed25519Identity) Unmarshal(key []byte) error {
	parsedKey, err := jwt.ParseEdPrivateKeyFromPEM(key)
	if err != nil {
		return fmt.Errorf("parsing Ed25519 private key: %w", err)
	}
	var privKey ed25519.PrivateKey
	var ok bool
	if privKey, ok = parsedKey.(ed25519.PrivateKey); !ok {
		return jwt.ErrNotEdPrivateKey
	}
	i.privateKey = privKey
	return nil
}
