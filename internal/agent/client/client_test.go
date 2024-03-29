package client

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Client Suite")
}

var _ = Describe("Elemental API Client Init", Label("agent", "client"), func() {
	var client Client
	var fs vfs.FS
	var err error
	var fsCleanup func()
	conf := config.Config{
		Registration: v1beta1.Registration{
			URI: "https://localhost:9090/just/for/testing",
			CACert: `-----BEGIN CERTIFICATE-----
MIIBvDCCAWOgAwIBAgIBADAKBggqhkjOPQQDAjBGMRwwGgYDVQQKExNkeW5hbWlj
bGlzdGVuZXItb3JnMSYwJAYDVQQDDB1keW5hbWljbGlzdGVuZXItY2FAMTY5NzEy
NjgwNTAeFw0yMzEwMTIxNjA2NDVaFw0zMzEwMDkxNjA2NDVaMEYxHDAaBgNVBAoT
E2R5bmFtaWNsaXN0ZW5lci1vcmcxJjAkBgNVBAMMHWR5bmFtaWNsaXN0ZW5lci1j
YUAxNjk3MTI2ODA1MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE9KvZXqQ7+hN/
4T0LVsFogfENa7UeSI3egvhg54qA6kI4ROQj0sObkbuBbepgGEcaOw8eJW0+M4o3
+SnprKYPkqNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYD
VR0OBBYEFD8W3gE6pK1EjnBM/kPaQF3Uqkc1MAoGCCqGSM49BAMCA0cAMEQCIDxz
wcHkvD3kEU33TR9VnkHUwgC9jDyDa62sef84S5MUAiAJfWf5G5PqtN+AE4XJgg2K
+ETPIs22tcmXyYOG0WY7KQ==
-----END CERTIFICATE-----`,
		},
	}
	var identity identity.Identity

	BeforeEach(func() {
		client = NewClient("v0.0.0-test")
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(fsCleanup)
	})
	It("should succeed on valid config", func() {
		Expect(client.Init(fs, identity, conf)).Should(Succeed())
	})
	It("should fail on http insecure protocol", func() {
		httpURIConf := conf
		httpURIConf.Registration.URI = "http://localhost:9090/just/for/testing"
		Expect(client.Init(fs, identity, httpURIConf)).Should(MatchError(ErrInvalidScheme))
		// Allow insecure http
		httpURIConf.Agent.InsecureAllowHTTP = true
		Expect(client.Init(fs, identity, httpURIConf)).Should(Succeed())
	})
	It("should fail on badly formatted CACert", func() {
		badCACertConf := conf
		badCACertConf.Registration.CACert = "not a parsable cert"
		Expect(client.Init(fs, identity, badCACertConf)).ShouldNot(Succeed())
	})
	It("should fail on badly formatted URI", func() {
		badURIConf := conf
		badURIConf.Registration.URI = "not a parsable URL"
		Expect(client.Init(fs, identity, badURIConf)).ShouldNot(Succeed())
	})
	It("should fail on unknown protocol", func() {
		unknownProtocolConf := conf
		unknownProtocolConf.Registration.URI = "unknown://localhost:9090/just/for/testing"
		Expect(client.Init(fs, identity, unknownProtocolConf)).Should(MatchError(ErrInvalidScheme))
		// Verify behavior when http allowed
		unknownProtocolConf.Agent.InsecureAllowHTTP = true
		Expect(client.Init(fs, identity, unknownProtocolConf)).Should(MatchError(ErrInvalidScheme))
	})
})
