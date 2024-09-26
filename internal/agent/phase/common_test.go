package phase

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	ConfigPathFixture = "/etc/just/for/test/config.yaml"

	ConfigFixture = config.Config{
		Registration: v1beta1.Registration{
			URI:    "https://test.test/elemental/v1/namespaces/test/registrations/test",
			CACert: "just a CA cert",
			Token:  "just a test token",
		},
		Agent: v1beta1.Agent{
			WorkDir: "/test/var/lib/elemental/agent",
			Hostname: v1beta1.Hostname{
				UseExisting: true,
				Prefix:      "test-",
			},
			Debug:                 true,
			NoSMBIOS:              true,
			OSPlugin:              "/a/mocked/plugin.so",
			Reconciliation:        time.Microsecond,
			InsecureAllowHTTP:     false,
			InsecureSkipTLSVerify: false,
			UseSystemCertPool:     false,
			PostInstall: v1beta1.PostAction{
				PowerOff: true,
				Reboot:   true, // If PowerOff is also true, this will be ignored
			},
			PostReset: v1beta1.PostAction{
				PowerOff: false,
				Reboot:   true,
			},
		},
	}

	RegistrationFixture = api.RegistrationResponse{
		HostLabels:      map[string]string{"test-label": "test"},
		HostAnnotations: map[string]string{"test-annotation": "test"},
		Config: v1beta1.Config{
			Elemental: v1beta1.Elemental{
				Registration: ConfigFixture.Registration,
				Agent:        ConfigFixture.Agent,
				Install: map[string]runtime.RawExtension{
					"firmware":         {Raw: []byte(`"test firmware"`)},
					"device":           {Raw: []byte(`"test device"`)},
					"noFormat":         {Raw: []byte("true")},
					"configUrls":       {Raw: []byte(`["test config url 1", "test config url 2"]`)},
					"iso":              {Raw: []byte(`"test iso"`)},
					"systemUri":        {Raw: []byte(`"test system uri"`)},
					"debug":            {Raw: []byte("true")},
					"tty":              {Raw: []byte(`"test tty"`)},
					"ejectCd":          {Raw: []byte("true")},
					"disableBootEntry": {Raw: []byte("true")},
					"configDir":        {Raw: []byte(`"test config dir"`)},
					// Not used, should be ignored.
					"poweroff": {Raw: []byte("true")},
					"reboot":   {Raw: []byte("true")},
				},
				Reset: map[string]runtime.RawExtension{
					"enabled":         {Raw: []byte("true")},
					"resetPersistent": {Raw: []byte("true")},
					"resetOem":        {Raw: []byte("true")},
					"configUrls":      {Raw: []byte(`["test config url 1", "test config url 2"]`)},
					"systemUri":       {Raw: []byte(`"test system uri"`)},
					"debug":           {Raw: []byte("true")},
					// Not used, should be ignored.
					"poweroff": {Raw: []byte("true")},
					"reboot":   {Raw: []byte("true")},
				},
			},
		},
	}

	OSVersionManagementFixture = map[string]runtime.RawExtension{
		"foo": {Raw: []byte(`"bar"`)},
		"bar": {Raw: []byte(`"foo"`)},
	}

	HostResponseFixture = api.HostResponse{
		Name:        "test-host",
		Annotations: map[string]string{"test-annotation": "test"},
		Labels:      map[string]string{"test-label": "test"},
	}
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Host Phases Suite")
}
