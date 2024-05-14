package main

import (
	"errors"
	"os"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
	"go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase/phases"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
)

func TestAgentCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elemental Agent CLI Suite")
}

var (
	configFixture = config.Config{
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
			PostInstall: v1beta1.PostInstall{
				PowerOff: true,
				Reboot:   true, // If PowerOff is also true, this will be ignored
			},
			PostReset: v1beta1.PostReset{
				PowerOff: false,
				Reboot:   true,
			},
		},
	}

	hostResponseFixture = api.HostResponse{
		Name:        "test-host",
		Annotations: map[string]string{"test-annotation": "test"},
		Labels:      map[string]string{"test-label": "test"},
	}
)

var _ = Describe("elemental-agent", Label("agent", "cli", "sanity"), func() {
	var fs vfs.FS
	var err error
	var fsCleanup func()
	var cmd *cobra.Command
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var pluginLoader *osplugin.MockLoader

	BeforeEach(func() {
		viper.Reset()
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		pluginLoader = osplugin.NewMockLoader(mockCtrl)
		cmd = newCommand(fs, pluginLoader, mClient, nil)
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
		cmd.SetArgs([]string{"--debug"})
		Expect(cmd.Execute()).To(HaveOccurred())
	})
	It("should fail if plugin can't be loaded", func() {
		marshalIntoFile(fs, configFixture, configPathDefault)
		cmd.SetArgs([]string{"--debug"})
		pluginLoader.EXPECT().Load(configFixture.Agent.OSPlugin).Return(nil, errors.New("test error"))
		Expect(cmd.Execute()).To(HaveOccurred())
	})
	It("should load custom config if --config argument", func() {
		alternateConfig := configFixture
		alternateConfig.Agent.OSPlugin = "a different plugin"
		const customConfigPath = "/test/etc/elemental/agent/config.yaml"
		marshalIntoFile(fs, alternateConfig, customConfigPath)
		cmd.SetArgs([]string{"--config", customConfigPath})
		// Let's make it fail to not go further.
		// Loading the plugin on the alternate path is already enough to verify the custom config is in use
		pluginLoader.EXPECT().Load(alternateConfig.Agent.OSPlugin).Return(nil, errors.New("test error"))
		Expect(cmd.Execute()).To(HaveOccurred())
	})
})

var _ = Describe("elemental-agent", Label("agent", "cli"), func() {
	var fs vfs.FS
	var err error
	var fsCleanup func()
	var cmd *cobra.Command
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var pluginLoader *osplugin.MockLoader
	var plugin *osplugin.MockPlugin
	var hostPhaseHandler *phase.MockHostPhaseHandler
	BeforeEach(func() {
		viper.Reset()
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		marshalIntoFile(fs, configFixture, configPathDefault)
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		pluginLoader = osplugin.NewMockLoader(mockCtrl)
		plugin = osplugin.NewMockPlugin(mockCtrl)
		hostPhaseHandler = phase.NewMockHostPhaseHandler(mockCtrl)
		gomock.InOrder(
			pluginLoader.EXPECT().Load(configFixture.Agent.OSPlugin).Return(plugin, nil),
			plugin.EXPECT().Init(osplugin.PluginContext{
				WorkDir:    configFixture.Agent.WorkDir,
				ConfigPath: configPathDefault,
				Debug:      configFixture.Agent.Debug,
			}).Return(nil),
			mClient.EXPECT().Init(fs, gomock.Any(), configFixture).Return(nil),
			plugin.EXPECT().GetHostname().Return(hostResponseFixture.Name, nil),
			hostPhaseHandler.EXPECT().Init(fs, mClient, plugin, gomock.Any(), phase.HostContext{
				AgentConfig:     configFixture,
				AgentConfigPath: configPathDefault,
				Hostname:        hostResponseFixture.Name,
			}),
		)

		cmd = newCommand(fs, pluginLoader, mClient, hostPhaseHandler)
		DeferCleanup(fsCleanup)
	})
	AfterEach(func() {
		// Ensure agent work dir was initialized
		workDir, err := fs.Stat(configFixture.Agent.WorkDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(workDir.IsDir()).Should(BeTrue())
	})
	When("operating normally", func() {
		triggerResetResponse := hostResponseFixture
		triggerResetResponse.NeedsReset = true

		triggerBootstrapResponse := hostResponseFixture
		triggerBootstrapResponse.BootstrapReady = true

		It("should trigger reset", func() {
			cmd.SetArgs([]string{"--debug"})
			gomock.InOrder(
				// Make first patch fail, expect to patch again
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
				// Make first reset attempt fail, expect to try again
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseTriggeringReset).Return(phases.PostCondition{}, errors.New("trigger reset test fail")),
				// Second trigger reset attempt, succeed
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseTriggeringReset).Return(phases.PostCondition{}, nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
		It("should bootstrap", func() {
			cmd.SetArgs([]string{"--debug"})
			gomock.InOrder(
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
				// Make first bootstrap fail, expect to try again
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseBootstrapping).Return(phases.PostCondition{}, errors.New("bootstrap fail")),
				// Second attempt, succeed
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseBootstrapping).Return(phases.PostCondition{Reboot: true}, nil),
				plugin.EXPECT().Reboot().Return(nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
		It("should prioritize triggering reset before anything else", func() {
			cmd.SetArgs([]string{"--debug"})
			bootstrapAndResetResponse := triggerResetResponse
			bootstrapAndResetResponse.BootstrapReady = true
			gomock.InOrder(
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&bootstrapAndResetResponse, nil),
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseTriggeringReset).Return(phases.PostCondition{}, nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
	})
	When("--register", func() {
		It("should register and exit", func() {
			cmd.SetArgs([]string{"--register"})
			gomock.InOrder(
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseRegistering).Return(phases.PostCondition{}, nil),
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseFinalizingRegistration).Return(phases.PostCondition{}, nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
		It("should register and try to install if --install also passed", func() {
			cmd.SetArgs([]string{"--register", "--install"})
			gomock.InOrder(
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseRegistering).Return(phases.PostCondition{}, nil),
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseFinalizingRegistration).Return(phases.PostCondition{}, nil),
				hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseInstalling).Return(phases.PostCondition{}, nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
		When("--install", func() {
			It("should install", func() {
				cmd.SetArgs([]string{"--install"})
				gomock.InOrder(
					hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseInstalling).Return(phases.PostCondition{PowerOff: true}, nil),
					// Post --install
					plugin.EXPECT().PowerOff().Return(nil),
				)
				Expect(cmd.Execute()).ToNot(HaveOccurred())
			})
		})
		When("--reset", func() {
			It("should reset", func() {
				cmd.SetArgs([]string{"--reset"})
				gomock.InOrder(
					hostPhaseHandler.EXPECT().Handle(infrastructurev1beta1.PhaseResetting).Return(phases.PostCondition{Reboot: true}, nil),
					// Post --reset
					plugin.EXPECT().Reboot().Return(nil),
				)
				Expect(cmd.Execute()).ToNot(HaveOccurred())
			})
		})
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
