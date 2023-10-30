package installer

import (
	"os"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	testWorkDir  = "/test/work/dir"
	testConfPath = "/test/config/path/config.yaml"
	testHostname = "just-a-test-hostname"
)

var (
	configFixture = config.Config{
		Registration: infrastructurev1beta1.Registration{
			URI:    "https://test.test/elemental/v1/namespaces/test/registrations/test",
			CACert: "just a CA cert",
		},
		Agent: infrastructurev1beta1.Agent{
			WorkDir: testWorkDir,
			Hostname: infrastructurev1beta1.Hostname{
				UseExisting: false,
				Prefix:      "test-",
			},
			Debug:                 true,
			NoSMBIOS:              true,
			Installer:             "test",
			Reconciliation:        time.Second,
			InsecureAllowHTTP:     false,
			InsecureSkipTLSVerify: false,
			UseSystemCertPool:     false,
		},
	}
	registrationFixture = api.RegistrationResponse{
		HostLabels:      map[string]string{"test-label": "test"},
		HostAnnotations: map[string]string{"test-annotation": "test"},
		Config: infrastructurev1beta1.Config{
			CloudConfig: map[string]runtime.RawExtension{
				"users": {
					Raw: []byte(`[{"name":"root","passwd":"root"}]`),
				},
			},
			Elemental: infrastructurev1beta1.Elemental{
				Registration: configFixture.Registration,
				Agent:        configFixture.Agent,
				Install: map[string]runtime.RawExtension{
					"foo": {
						Raw: []byte(`{"bar":{"foobar":"barfoo"}}`),
					},
				},
				Reset: map[string]runtime.RawExtension{
					"foo": {
						Raw: []byte(`{"bar":{"foobar":"barfoo"}}`),
					},
				},
			},
		},
	}
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Installer Suite")
}

func marshalIntoFile(fs vfs.FS, input any, filePath string) {
	bytes := marshalToBytes(input)
	Expect(vfs.MkdirAll(fs, path.Dir(filePath), os.ModePerm)).ToNot(HaveOccurred())
	Expect(fs.WriteFile(filePath, bytes, os.ModePerm)).ToNot(HaveOccurred())
}

func marshalToBytes(input any) []byte {
	bytes, err := yaml.Marshal(input)
	Expect(err).ToNot(HaveOccurred())
	return bytes
}

func compareFiles(fs vfs.FS, got string, want string) {
	gotFile, err := fs.ReadFile(got)
	Expect(err).ToNot(HaveOccurred())
	wantFile, err := os.ReadFile(want)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gotFile)).To(Equal(string(wantFile)))
}
