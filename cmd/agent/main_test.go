package main

import (
	"os"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/installer"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	"go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"
)

func TestRegister(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elemental Agent CLI Suite")
}

var (
	configFixture = config.Config{
		Registration: infrastructurev1beta1.Registration{
			URI:    "https://test.test/elemental/v1/namespaces/test/registrations/test",
			CACert: "just a CA cert",
		},
		Agent: infrastructurev1beta1.Agent{
			WorkDir: "/test/var/lib/elemental/agent",
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
			Elemental: infrastructurev1beta1.Elemental{
				Registration: configFixture.Registration,
				Agent:        configFixture.Agent,
			},
		},
	}

	hostResponseFixture = api.HostResponse{
		Name:        "test-host",
		Annotations: map[string]string{"test-annotation": "test"},
		Labels:      map[string]string{"test-label": "test"},
	}
)

var _ = Describe("elemental-agent", Label("agent", "cli"), func() {
	var fs vfs.FS
	var err error
	var fsCleanup func()
	var cmd *cobra.Command
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var mInstallerSelector *installer.MockInstallerSelector
	var mInstaller *installer.MockInstaller
	var mHostManager *host.MockManager
	BeforeEach(func() {
		viper.Reset()
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		mInstallerSelector = installer.NewMockInstallerSelector(mockCtrl)
		mInstaller = installer.NewMockInstaller(mockCtrl)
		mHostManager = host.NewMockManager(mockCtrl)
		cmd = newCommand(fs, mInstallerSelector, mHostManager, mClient)
		DeferCleanup(fsCleanup)
	})
	It("should return no error when printing version", func() {
		cmd.SetArgs([]string{"--version"})
		Expect(cmd.Execute()).ToNot(HaveOccurred())
	})
	It("should fail if --install and --reset used together", func() {
		cmd.SetArgs([]string{"--install", "--reset"})
		Expect(cmd.Execute()).To(HaveOccurred())
	})
	It("should fail if no default config file exists", func() {
		Expect(cmd.Execute()).To(HaveOccurred())
	})
	It("should use custom config if --config argument", func() {
		const customConfigPath = "/test/etc/elemental/agent/config.yaml"
		marshalIntoFile(fs, configFixture, customConfigPath)
		cmd.SetArgs([]string{"--config", customConfigPath})
		mHostManager.EXPECT().GetCurrentHostname().Return(hostResponseFixture.Name, nil)
		mClient.EXPECT().Init(fs, configFixture).Return(nil)
		mInstallerSelector.EXPECT().GetInstaller(fs, customConfigPath, configFixture).Return(mInstaller, nil)
		triggerResetResponse := hostResponseFixture
		triggerResetResponse.NeedsReset = true
		mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(triggerResetResponse, nil)
		mInstaller.EXPECT().TriggerReset().Return(nil)
		Expect(cmd.Execute()).ToNot(HaveOccurred())
	})
	It("should install when --install", func() {
		marshalIntoFile(fs, configFixture, configPathDefault)
		cmd.SetArgs([]string{"--install"})
		mHostManager.EXPECT().GetCurrentHostname().Return(hostResponseFixture.Name, nil)
		mClient.EXPECT().Init(fs, configFixture).Return(nil)
		mInstallerSelector.EXPECT().GetInstaller(fs, configPathDefault, configFixture).Return(mInstaller, nil)
		mClient.EXPECT().GetRegistration().Return(registrationFixture, nil)
		mHostManager.EXPECT().PickHostname(configFixture.Agent.Hostname).Return(hostResponseFixture.Name, nil)
		mClient.EXPECT().CreateHost(api.HostCreateRequest{
			Name:        hostResponseFixture.Name,
			Annotations: registrationFixture.HostAnnotations,
			Labels:      registrationFixture.HostLabels,
		}).Return(nil)
		mInstaller.EXPECT().Install(registrationFixture, hostResponseFixture.Name).Return(nil)
		mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
			if patch.Installed == nil {
				GinkgoT().Error("installation patch does not contain installed flag")
			}
			if !*patch.Installed {
				GinkgoT().Error("installation patch does not contain true installed flag")
			}
		})
		Expect(cmd.Execute()).ToNot(HaveOccurred())
	})
	It("should reset when --reset", func() {
		marshalIntoFile(fs, configFixture, configPathDefault)
		cmd.SetArgs([]string{"--reset"})
		mHostManager.EXPECT().GetCurrentHostname().Return(hostResponseFixture.Name, nil)
		mClient.EXPECT().Init(fs, configFixture).Return(nil)
		mInstallerSelector.EXPECT().GetInstaller(fs, configPathDefault, configFixture).Return(mInstaller, nil)
		mClient.EXPECT().GetRegistration().Return(registrationFixture, nil)
		mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil)
		mInstaller.EXPECT().Reset(registrationFixture).Return(nil)
		mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
			if patch.Reset == nil {
				GinkgoT().Error("reset patch does not contain reset flag")
			}
			if !*patch.Reset {
				GinkgoT().Error("reset patch does not contain true reset flag")
			}
		})
		Expect(cmd.Execute()).ToNot(HaveOccurred())
	})
})

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
