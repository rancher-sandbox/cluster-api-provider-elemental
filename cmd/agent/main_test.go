package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	"go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRegister(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elemental Agent CLI Suite")
}

var (
	configFixture = config.Config{
		Registration: v1beta1.Registration{
			URI:    "https://test.test/elemental/v1/namespaces/test/registrations/test",
			CACert: "just a CA cert",
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

	registrationFixture = &api.RegistrationResponse{
		HostLabels:      map[string]string{"test-label": "test"},
		HostAnnotations: map[string]string{"test-annotation": "test"},
		Config: v1beta1.Config{
			Elemental: v1beta1.Elemental{
				Registration: configFixture.Registration,
				Agent:        configFixture.Agent,
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

	hostResponseFixture = &api.HostResponse{
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
	var cmdRunner *utils.MockCommandRunner
	BeforeEach(func() {
		viper.Reset()
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		pluginLoader = osplugin.NewMockLoader(mockCtrl)
		cmdRunner = utils.NewMockCommandRunner(mockCtrl)
		cmd = newCommand(fs, pluginLoader, cmdRunner, mClient)
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
	var cmdRunner *utils.MockCommandRunner
	var pluginLoader *osplugin.MockLoader
	var plugin *osplugin.MockPlugin
	BeforeEach(func() {
		viper.Reset()
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		marshalIntoFile(fs, configFixture, configPathDefault)
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		pluginLoader = osplugin.NewMockLoader(mockCtrl)
		plugin = osplugin.NewMockPlugin(mockCtrl)
		gomock.InOrder(
			pluginLoader.EXPECT().Load(configFixture.Agent.OSPlugin).Return(plugin, nil),
			plugin.EXPECT().Init(osplugin.PluginContext{
				WorkDir:    configFixture.Agent.WorkDir,
				ConfigPath: configPathDefault,
				Debug:      configFixture.Agent.Debug,
			}).Return(nil),
			mClient.EXPECT().Init(fs, gomock.Any(), configFixture).Return(nil),
			plugin.EXPECT().GetHostname().Return(hostResponseFixture.Name, nil),
		)
		cmdRunner = utils.NewMockCommandRunner(mockCtrl)
		cmd = newCommand(fs, pluginLoader, cmdRunner, mClient)
		DeferCleanup(fsCleanup)
	})
	AfterEach(func() {
		// Ensure agent work dir was initialized
		workDir, err := fs.Stat(configFixture.Agent.WorkDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(workDir.IsDir()).Should(BeTrue())
	})
	When("operating normally", func() {
		triggerResetResponse := *hostResponseFixture
		triggerResetResponse.NeedsReset = true

		triggerBootstrapResponse := *hostResponseFixture
		triggerBootstrapResponse.BootstrapReady = true

		bootstrapResponse := api.BootstrapResponse{
			Files: []api.WriteFile{
				{
					Path:        "/foo",
					Owner:       "foo:foo",
					Permissions: "0640",
					Content:     "foo/n",
				},
				{
					Path:        "/bar",
					Owner:       "bar:bar",
					Permissions: "0640",
					Content:     "bar/n",
				},
			},
			Commands: []string{"foo", "bar"},
		}
		It("should trigger reset", func() {
			cmd.SetArgs([]string{"--debug"})
			gomock.InOrder(
				// Make first patch fail, expect to patch again
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
				// Make first reset attempt fail, expect to try again
				plugin.EXPECT().TriggerReset().Return(errors.New("trigger reset test fail")),
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
				plugin.EXPECT().TriggerReset().Return(nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
		It("should bootstrap", func() {
			cmd.SetArgs([]string{"--debug"})
			gomock.InOrder(
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
				// Make first get bootstrap fail, expect to try again
				mClient.EXPECT().GetBootstrap(hostResponseFixture.Name).Return(nil, errors.New("get bootstrap test fail")),
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
				mClient.EXPECT().GetBootstrap(hostResponseFixture.Name).Return(&bootstrapResponse, nil),
				// Verify commands are executed
				cmdRunner.EXPECT().RunCommand("foo").Return(nil),
				cmdRunner.EXPECT().RunCommand("bar").Return(nil),
				// Expect bootstrapped patch
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
					if patch.Bootstrapped == nil {
						GinkgoT().Error("bootstrapped patch does not contain bootstrapped flag")
					}
					if !*patch.Bootstrapped {
						GinkgoT().Error("bootstrapped patch does not contain true bootstrapped flag")
					}
				}),
				// Trigger reset just to exit the program (and the test)
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
				plugin.EXPECT().TriggerReset().Return(nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
			// Verify bootstrap files are created
			Expect(fs.ReadFile("/foo")).Should(Equal([]byte("foo/n")))
			Expect(fs.ReadFile("/bar")).Should(Equal([]byte("bar/n")))
		})
		It("should not bootstrap twice", func() {
			cmd.SetArgs([]string{"--debug"})
			// Mark the system as bootstrapped. This path is part of the CAPI contract: https://cluster-api.sigs.k8s.io/developer/providers/bootstrap.html#sentinel-file
			Expect(vfs.MkdirAll(fs, "/run/cluster-api", os.ModePerm)).Should(Succeed())
			Expect(fs.WriteFile("/run/cluster-api/bootstrap-success.complete", []byte("anything"), os.ModePerm)).Should(Succeed())
			gomock.InOrder(
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
				// Expect bootstrapped patch
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
					if patch.Bootstrapped == nil {
						GinkgoT().Error("bootstrapped patch does not contain bootstrapped flag")
					}
					if !*patch.Bootstrapped {
						GinkgoT().Error("bootstrapped patch does not contain true bootstrapped flag")
					}
				}),
				// Trigger reset just to exit the program (and the test)
				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
				plugin.EXPECT().TriggerReset().Return(nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
	})
	When("--register", func() {
		wantCreateHostRequest := api.HostCreateRequest{
			Name:        hostResponseFixture.Name,
			Annotations: registrationFixture.HostAnnotations,
			Labels:      registrationFixture.HostLabels,
		}
		wantAgentConfig := config.FromAPI(registrationFixture)
		wantAgentConfigBytes, err := yaml.Marshal(wantAgentConfig)
		wantIdentityFilePath := fmt.Sprintf("%s/%s", registrationFixture.Config.Elemental.Agent.WorkDir, identity.PrivateKeyFile)
		It("should register and exit", func() {
			Expect(err).ToNot(HaveOccurred())
			cmd.SetArgs([]string{"--register"})
			gomock.InOrder(
				// First get registration call fails. Should repeat to recover.
				mClient.EXPECT().GetRegistration().Return(nil, errors.New("test get registration fail")),
				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
				plugin.EXPECT().GetHostname().Return("host", nil),
				// Let's make the first create host call fail. Expect to recover.
				mClient.EXPECT().CreateHost(wantCreateHostRequest).Return(errors.New("test creat host fail")),
				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
				// Expect a new hostname to be formatted due to creation failure.
				plugin.EXPECT().GetHostname().Return("host", nil),
				mClient.EXPECT().CreateHost(wantCreateHostRequest).Return(nil),
				// Post --register
				plugin.EXPECT().PersistHostname(hostResponseFixture.Name).Return(nil),
				plugin.EXPECT().PersistFile(wantAgentConfigBytes, configPathDefault, uint32(0640), 0, 0).Return(nil),
				plugin.EXPECT().PersistFile(gomock.Any(), wantIdentityFilePath, uint32(0640), 0, 0).Return(nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())
		})
		It("should register and try to install if --install also passed", func() {
			cmd.SetArgs([]string{"--register", "--install"})
			gomock.InOrder(
				// --register
				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
				plugin.EXPECT().GetHostname().Return("host", nil),
				mClient.EXPECT().CreateHost(wantCreateHostRequest).Return(nil),
				// Post --register
				plugin.EXPECT().PersistHostname(hostResponseFixture.Name).Return(nil),
				plugin.EXPECT().PersistFile(wantAgentConfigBytes, configPathDefault, uint32(0640), 0, 0).Return(nil),
				plugin.EXPECT().PersistFile(gomock.Any(), wantIdentityFilePath, uint32(0640), 0, 0).Return(nil),
				// --install
				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
				plugin.EXPECT().ApplyCloudInit(gomock.Any()).Return(nil),
				plugin.EXPECT().Install(gomock.Any()).Return(nil),
				mClient.EXPECT().PatchHost(gomock.Any(), gomock.Any()).Return(&api.HostResponse{}, nil),
				// post --install
				plugin.EXPECT().PowerOff().Return(nil),
			)
			Expect(cmd.Execute()).ToNot(HaveOccurred())

		})
		When("--install", func() {
			It("should apply cloud init, install, and mark the host as installed", func() {
				marshalIntoFile(fs, configFixture, configPathDefault)
				wantCloudInit, err := json.Marshal(registrationFixture.Config.CloudConfig)
				Expect(err).ToNot(HaveOccurred())
				wantInstall, err := json.Marshal(registrationFixture.Config.Elemental.Install)
				Expect(err).ToNot(HaveOccurred())
				cmd.SetArgs([]string{"--install"})
				gomock.InOrder(
					// Make the first get registration call fail. Expect to recover by calling again
					mClient.EXPECT().GetRegistration().Return(nil, errors.New("get registration test error")),
					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
					// Make the cloud init apply fail. Expect to recover by getting registration and applying cloud init again
					plugin.EXPECT().ApplyCloudInit(wantCloudInit).Return(errors.New("cloud init test failed")),
					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
					plugin.EXPECT().ApplyCloudInit(wantCloudInit).Return(nil),
					// Make the install fail. Expect to recover by getting registration and installing again
					plugin.EXPECT().Install(wantInstall).Return(errors.New("install test fail")),
					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
					plugin.EXPECT().Install(wantInstall).Return(nil),
					// Make the patch host fail. Expect to recover by patching it again
					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
						if patch.Installed == nil {
							GinkgoT().Error("installation patch does not contain installed flag")
						}
						if !*patch.Installed {
							GinkgoT().Error("installation patch does not contain true installed flag")
						}
					}),
					// Post --install
					plugin.EXPECT().PowerOff().Return(nil),
				)
				Expect(cmd.Execute()).ToNot(HaveOccurred())
			})
		})
		When("--reset", func() {
			It("should delete host, reset, and patch the host as reset", func() {
				marshalIntoFile(fs, configFixture, configPathDefault)
				wantReset, err := json.Marshal(registrationFixture.Config.Elemental.Reset)
				Expect(err).ToNot(HaveOccurred())
				cmd.SetArgs([]string{"--reset"})
				gomock.InOrder(
					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(errors.New("delete host test error")),
					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil),
					// Make the first registration call fail. Expect to recover by calling again
					mClient.EXPECT().GetRegistration().Return(nil, errors.New("get registration test error")),
					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil), // Always called
					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
					// Make the reset call fail. Expect to recover by getting registration and resetting again
					plugin.EXPECT().Reset(wantReset).Return(errors.New("reset test error")),
					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil),
					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
					plugin.EXPECT().Reset(wantReset).Return(nil),
					// Make the patch host fail. Expect to recover by patching it again
					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil),
					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
						if patch.Reset == nil {
							GinkgoT().Error("reset patch does not contain reset flag")
						}
						if !*patch.Reset {
							GinkgoT().Error("reset patch does not contain true reset flag")
						}
					}),
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
