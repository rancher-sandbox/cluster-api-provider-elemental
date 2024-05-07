package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elemental Agent CLI Suite")
}

var _ = Describe("elemental-agent", Label("agent", "cli", "sanity"), func() {
	It("should promise to refactor tests too", func() {
	})
})

// import (
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"os"
// 	"path"
// 	"path/filepath"
// 	"testing"
// 	"time"

// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	"github.com/spf13/cobra"
// 	"github.com/spf13/viper"
// 	"github.com/twpayne/go-vfs/v4"
// 	"github.com/twpayne/go-vfs/v4/vfst"
// 	"go.uber.org/mock/gomock"
// 	"gopkg.in/yaml.v3"
// 	corev1 "k8s.io/api/core/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// 	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
// 	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
// 	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
// 	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
// 	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
// 	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
// 	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
// )

// func TestAgentCLI(t *testing.T) {
// 	RegisterFailHandler(Fail)
// 	RunSpecs(t, "Elemental Agent CLI Suite")
// }

// var (
// 	configFixture = config.Config{
// 		Registration: v1beta1.Registration{
// 			URI:    "https://test.test/elemental/v1/namespaces/test/registrations/test",
// 			CACert: "just a CA cert",
// 			Token:  "just a test token",
// 		},
// 		Agent: v1beta1.Agent{
// 			WorkDir: "/test/var/lib/elemental/agent",
// 			Hostname: v1beta1.Hostname{
// 				UseExisting: true,
// 				Prefix:      "test-",
// 			},
// 			Debug:                 true,
// 			NoSMBIOS:              true,
// 			OSPlugin:              "/a/mocked/plugin.so",
// 			Reconciliation:        time.Microsecond,
// 			InsecureAllowHTTP:     false,
// 			InsecureSkipTLSVerify: false,
// 			UseSystemCertPool:     false,
// 			PostInstall: v1beta1.PostInstall{
// 				PowerOff: true,
// 				Reboot:   true, // If PowerOff is also true, this will be ignored
// 			},
// 			PostReset: v1beta1.PostReset{
// 				PowerOff: false,
// 				Reboot:   true,
// 			},
// 		},
// 	}

// 	registrationFixture = &api.RegistrationResponse{
// 		HostLabels:      map[string]string{"test-label": "test"},
// 		HostAnnotations: map[string]string{"test-annotation": "test"},
// 		Config: v1beta1.Config{
// 			Elemental: v1beta1.Elemental{
// 				Registration: configFixture.Registration,
// 				Agent:        configFixture.Agent,
// 				Install: map[string]runtime.RawExtension{
// 					"firmware":         {Raw: []byte(`"test firmware"`)},
// 					"device":           {Raw: []byte(`"test device"`)},
// 					"noFormat":         {Raw: []byte("true")},
// 					"configUrls":       {Raw: []byte(`["test config url 1", "test config url 2"]`)},
// 					"iso":              {Raw: []byte(`"test iso"`)},
// 					"systemUri":        {Raw: []byte(`"test system uri"`)},
// 					"debug":            {Raw: []byte("true")},
// 					"tty":              {Raw: []byte(`"test tty"`)},
// 					"ejectCd":          {Raw: []byte("true")},
// 					"disableBootEntry": {Raw: []byte("true")},
// 					"configDir":        {Raw: []byte(`"test config dir"`)},
// 					// Not used, should be ignored.
// 					"poweroff": {Raw: []byte("true")},
// 					"reboot":   {Raw: []byte("true")},
// 				},
// 				Reset: map[string]runtime.RawExtension{
// 					"enabled":         {Raw: []byte("true")},
// 					"resetPersistent": {Raw: []byte("true")},
// 					"resetOem":        {Raw: []byte("true")},
// 					"configUrls":      {Raw: []byte(`["test config url 1", "test config url 2"]`)},
// 					"systemUri":       {Raw: []byte(`"test system uri"`)},
// 					"debug":           {Raw: []byte("true")},
// 					// Not used, should be ignored.
// 					"poweroff": {Raw: []byte("true")},
// 					"reboot":   {Raw: []byte("true")},
// 				},
// 			},
// 		},
// 	}

// 	hostResponseFixture = &api.HostResponse{
// 		Name:        "test-host",
// 		Annotations: map[string]string{"test-annotation": "test"},
// 		Labels:      map[string]string{"test-label": "test"},
// 	}
// )

// var _ = Describe("elemental-agent", Label("agent", "cli", "sanity"), func() {
// 	var fs vfs.FS
// 	var err error
// 	var fsCleanup func()
// 	var cmd *cobra.Command
// 	var mockCtrl *gomock.Controller
// 	var mClient *client.MockClient
// 	var pluginLoader *osplugin.MockLoader
// 	BeforeEach(func() {
// 		viper.Reset()
// 		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
// 		Expect(err).ToNot(HaveOccurred())
// 		mockCtrl = gomock.NewController(GinkgoT())
// 		mClient = client.NewMockClient(mockCtrl)
// 		pluginLoader = osplugin.NewMockLoader(mockCtrl)
// 		cmd = newCommand(fs, pluginLoader, mClient)
// 		DeferCleanup(fsCleanup)
// 	})
// 	It("should return no error when printing version", func() {
// 		cmd.SetArgs([]string{"--version"})
// 		Expect(cmd.Execute()).ToNot(HaveOccurred())
// 	})
// 	It("should fail if --install and --reset used together", func() {
// 		cmd.SetArgs([]string{"--install", "--reset"})
// 		Expect(cmd.Execute()).To(HaveOccurred())
// 	})
// 	It("should fail if no default config file exists", func() {
// 		cmd.SetArgs([]string{"--debug"})
// 		Expect(cmd.Execute()).To(HaveOccurred())
// 	})
// 	It("should fail if plugin can't be loaded", func() {
// 		marshalIntoFile(fs, configFixture, configPathDefault)
// 		cmd.SetArgs([]string{"--debug"})
// 		pluginLoader.EXPECT().Load(configFixture.Agent.OSPlugin).Return(nil, errors.New("test error"))
// 		Expect(cmd.Execute()).To(HaveOccurred())
// 	})
// 	It("should load custom config if --config argument", func() {
// 		alternateConfig := configFixture
// 		alternateConfig.Agent.OSPlugin = "a different plugin"
// 		const customConfigPath = "/test/etc/elemental/agent/config.yaml"
// 		marshalIntoFile(fs, alternateConfig, customConfigPath)
// 		cmd.SetArgs([]string{"--config", customConfigPath})
// 		// Let's make it fail to not go further.
// 		// Loading the plugin on the alternate path is already enough to verify the custom config is in use
// 		pluginLoader.EXPECT().Load(alternateConfig.Agent.OSPlugin).Return(nil, errors.New("test error"))
// 		Expect(cmd.Execute()).To(HaveOccurred())
// 	})
// })

// var _ = Describe("elemental-agent", Label("agent", "cli"), func() {
// 	var fs vfs.FS
// 	var err error
// 	var fsCleanup func()
// 	var cmd *cobra.Command
// 	var mockCtrl *gomock.Controller
// 	var mClient *client.MockClient
// 	var pluginLoader *osplugin.MockLoader
// 	var plugin *osplugin.MockPlugin
// 	BeforeEach(func() {
// 		viper.Reset()
// 		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
// 		Expect(err).ToNot(HaveOccurred())
// 		marshalIntoFile(fs, configFixture, configPathDefault)
// 		mockCtrl = gomock.NewController(GinkgoT())
// 		mClient = client.NewMockClient(mockCtrl)
// 		pluginLoader = osplugin.NewMockLoader(mockCtrl)
// 		plugin = osplugin.NewMockPlugin(mockCtrl)
// 		gomock.InOrder(
// 			pluginLoader.EXPECT().Load(configFixture.Agent.OSPlugin).Return(plugin, nil),
// 			plugin.EXPECT().Init(osplugin.PluginContext{
// 				WorkDir:    configFixture.Agent.WorkDir,
// 				ConfigPath: configPathDefault,
// 				Debug:      configFixture.Agent.Debug,
// 			}).Return(nil),
// 			mClient.EXPECT().Init(fs, gomock.Any(), configFixture).Return(nil),
// 			plugin.EXPECT().GetHostname().Return(hostResponseFixture.Name, nil),
// 		)
// 		cmd = newCommand(fs, pluginLoader, mClient)
// 		DeferCleanup(fsCleanup)
// 	})
// 	AfterEach(func() {
// 		// Ensure agent work dir was initialized
// 		workDir, err := fs.Stat(configFixture.Agent.WorkDir)
// 		Expect(err).NotTo(HaveOccurred())
// 		Expect(workDir.IsDir()).Should(BeTrue())
// 	})
// 	When("operating normally", func() {
// 		triggerResetResponse := *hostResponseFixture
// 		triggerResetResponse.NeedsReset = true

// 		triggerBootstrapResponse := *hostResponseFixture
// 		triggerBootstrapResponse.BootstrapReady = true

// 		bootstrapResponse := api.BootstrapResponse{
// 			Format: "foo",
// 			Config: "bar",
// 		}
// 		It("should trigger reset", func() {
// 			cmd.SetArgs([]string{"--debug"})
// 			gomock.InOrder(
// 				// Make first patch fail, expect to patch again
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
// 				// Make first reset attempt fail, expect to try again
// 				plugin.EXPECT().TriggerReset().Return(errors.New("trigger reset test fail")),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.ResetReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: clusterv1.ConditionSeverityError,
// 							Reason:   infrastructurev1beta1.ResetFailedReason,
// 							Message:  "triggering reset: trigger reset test fail",
// 						},
// 					))
// 				}),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
// 				plugin.EXPECT().TriggerReset().Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.ResetReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: clusterv1.ConditionSeverityInfo,
// 							Reason:   infrastructurev1beta1.WaitingForResetReason,
// 							Message:  "Reset was triggered successfully. Waiting for host to reset.",
// 						},
// 					))
// 				}),
// 			)
// 			Expect(cmd.Execute()).ToNot(HaveOccurred())
// 		})
// 		It("should bootstrap when bootstrap sentinel file missing", func() {
// 			cmd.SetArgs([]string{"--debug"})
// 			gomock.InOrder(
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
// 				// Make first get bootstrap fail, expect to try again
// 				mClient.EXPECT().GetBootstrap(hostResponseFixture.Name).Return(nil, errors.New("get bootstrap test fail")),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.BootstrapReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: clusterv1.ConditionSeverityError,
// 							Reason:   infrastructurev1beta1.BootstrapFailedReason,
// 							Message:  "fetching bootstrap config: get bootstrap test fail",
// 						},
// 					))
// 				}),
// 				// Make bootstrap application fail on second attempt
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
// 				mClient.EXPECT().GetBootstrap(hostResponseFixture.Name).Return(&bootstrapResponse, nil),
// 				plugin.EXPECT().Bootstrap(bootstrapResponse.Format, []byte(bootstrapResponse.Config)).Return(errors.New("apply bootstrap test fail")),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.BootstrapReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: clusterv1.ConditionSeverityError,
// 							Reason:   infrastructurev1beta1.BootstrapFailedReason,
// 							Message:  "applying bootstrap config: apply bootstrap test fail",
// 						},
// 					))
// 				}),
// 				// Third time's a charm
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
// 				mClient.EXPECT().GetBootstrap(hostResponseFixture.Name).Return(&bootstrapResponse, nil),
// 				plugin.EXPECT().Bootstrap(bootstrapResponse.Format, []byte(bootstrapResponse.Config)).Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.BootstrapReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: infrastructurev1beta1.WaitingForBootstrapReasonSeverity,
// 							Reason:   infrastructurev1beta1.WaitingForBootstrapReason,
// 							Message:  "Waiting for bootstrap to be executed",
// 						},
// 					))
// 				}),
// 				plugin.EXPECT().Reboot().Return(nil),
// 				// Program should exit after reboot
// 			)
// 			Expect(cmd.Execute()).ToNot(HaveOccurred())
// 		})
// 		It("should exit program when post bootstrap reboot fails", func() {
// 			cmd.SetArgs([]string{"--debug"})
// 			gomock.InOrder(
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
// 				mClient.EXPECT().GetBootstrap(hostResponseFixture.Name).Return(&bootstrapResponse, nil),
// 				plugin.EXPECT().Bootstrap(bootstrapResponse.Format, []byte(bootstrapResponse.Config)).Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.BootstrapReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: infrastructurev1beta1.WaitingForBootstrapReasonSeverity,
// 							Reason:   infrastructurev1beta1.WaitingForBootstrapReason,
// 							Message:  "Waiting for bootstrap to be executed",
// 						},
// 					))
// 				}),
// 				plugin.EXPECT().Reboot().Return(errors.New("reboot test fail")),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.BootstrapReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: clusterv1.ConditionSeverityError,
// 							Reason:   infrastructurev1beta1.BootstrapFailedReason,
// 							Message:  "rebooting system for bootstrapping: reboot test fail",
// 						},
// 					))
// 				}),
// 				// Program should exit on reboot failure
// 				// Currently this is intended behavior, as we don't expect to recover from reboot errors.
// 				// Potentially this can be improved by re-invoking reboot on failures, but beware of:
// 				// - Not re-applying bootstrap twice (plugin.Bootstrap)
// 				// - Not being stuck in an endless recovery loop, and allow reset to be triggered at any time as a way out remote solution
// 			)
// 			Expect(cmd.Execute()).ToNot(HaveOccurred())
// 		})
// 		It("should patch the host as bootstrapped when sentinel file is present", func() {
// 			cmd.SetArgs([]string{"--debug"})
// 			// Mark the system as bootstrapped. This path is part of the CAPI contract: https://cluster-api.sigs.k8s.io/developer/providers/bootstrap.html#sentinel-file
// 			Expect(vfs.MkdirAll(fs, "/run/cluster-api", os.ModePerm)).Should(Succeed())
// 			Expect(fs.WriteFile("/run/cluster-api/bootstrap-success.complete", []byte("anything"), os.ModePerm)).Should(Succeed())
// 			gomock.InOrder(
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
// 				// Make first patch attempt fail
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("bootstrapped patch test fail")),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.BootstrapReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: clusterv1.ConditionSeverityError,
// 							Reason:   infrastructurev1beta1.BootstrapFailedReason,
// 							Message:  "patching ElementalHost after bootstrap: bootstrapped patch test fail",
// 						},
// 					))
// 				}),
// 				// Succeed on second attempt
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerBootstrapResponse, nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					if patch.Bootstrapped == nil {
// 						GinkgoT().Error("bootstrapped patch does not contain bootstrapped flag")
// 					}
// 					if !*patch.Bootstrapped {
// 						GinkgoT().Error("bootstrapped patch does not contain true bootstrapped flag")
// 					}
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.BootstrapReady,
// 							Status:   corev1.ConditionTrue,
// 							Severity: clusterv1.ConditionSeverityInfo,
// 							Reason:   "",
// 							Message:  "",
// 						},
// 					))
// 				}),
// 				// Trigger reset just to exit the program (and the test)
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&triggerResetResponse, nil),
// 				plugin.EXPECT().TriggerReset().Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil), // condition reporting
// 			)
// 			Expect(cmd.Execute()).ToNot(HaveOccurred())
// 		})
// 		It("should trigger reset before boostrap", func() {
// 			cmd.SetArgs([]string{"--debug"})
// 			bootstrapAndResetResponse := triggerResetResponse
// 			bootstrapAndResetResponse.BootstrapReady = true
// 			gomock.InOrder(
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(&bootstrapAndResetResponse, nil),
// 				plugin.EXPECT().TriggerReset().Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil), // condition reporting
// 				// Implicitly any other call to the mocked plugin will make the test fail.
// 			)
// 			Expect(cmd.Execute()).ToNot(HaveOccurred())
// 		})
// 	})
// 	When("--register", func() {
// 		wantCreateHostRequest := api.HostCreateRequest{
// 			Name:        hostResponseFixture.Name,
// 			Annotations: registrationFixture.HostAnnotations,
// 			Labels:      registrationFixture.HostLabels,
// 		}
// 		wantAgentConfig := config.FromAPI(registrationFixture)
// 		wantAgentConfigBytes, err := yaml.Marshal(wantAgentConfig)
// 		Expect(err).ToNot(HaveOccurred())
// 		wantIdentityFilePath := fmt.Sprintf("%s/%s", registrationFixture.Config.Elemental.Agent.WorkDir, identity.PrivateKeyFile)
// 		It("should register and exit", func() {
// 			_, pubKeyPem := initializeIdentity(fs)
// 			wantRequest := wantCreateHostRequest
// 			wantRequest.PubKey = pubKeyPem
// 			cmd.SetArgs([]string{"--register"})
// 			gomock.InOrder(
// 				// First get registration call fails. Should repeat to recover.
// 				mClient.EXPECT().GetRegistration().Return(nil, errors.New("test get registration fail")),
// 				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 				plugin.EXPECT().GetHostname().Return("host", nil),
// 				// Let's make the first create host call fail. Expect to recover.
// 				mClient.EXPECT().CreateHost(wantRequest).Return(errors.New("test creat host fail")),
// 				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 				// Expect a new hostname to be formatted due to creation failure.
// 				plugin.EXPECT().GetHostname().Return("host", nil),
// 				mClient.EXPECT().CreateHost(wantRequest).Return(nil),
// 				// Post --register
// 				plugin.EXPECT().InstallHostname(hostResponseFixture.Name).Return(nil),
// 				plugin.EXPECT().InstallFile(wantAgentConfigBytes, configPathDefault, uint32(0640), 0, 0).Return(nil),
// 				plugin.EXPECT().InstallFile(gomock.Any(), wantIdentityFilePath, uint32(0640), 0, 0).Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.RegistrationReady,
// 							Status:   corev1.ConditionTrue,
// 							Severity: clusterv1.ConditionSeverityInfo,
// 							Reason:   "",
// 							Message:  "",
// 						},
// 					))
// 				}),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 					Expect(*patch.Condition).Should(Equal(
// 						clusterv1.Condition{
// 							Type:     infrastructurev1beta1.InstallationReady,
// 							Status:   corev1.ConditionFalse,
// 							Severity: infrastructurev1beta1.WaitingForInstallationReasonSeverity,
// 							Reason:   infrastructurev1beta1.WaitingForInstallationReason,
// 							Message:  "Host is registered successfully. Waiting for installation.",
// 						},
// 					))
// 				}),
// 			)
// 			Expect(cmd.Execute()).ToNot(HaveOccurred())
// 		})
// 		It("should register and try to install if --install also passed", func() {
// 			_, pubKeyPem := initializeIdentity(fs)
// 			wantRequest := wantCreateHostRequest
// 			wantRequest.PubKey = pubKeyPem
// 			cmd.SetArgs([]string{"--register", "--install"})
// 			gomock.InOrder(
// 				// --register
// 				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 				plugin.EXPECT().GetHostname().Return("host", nil),
// 				mClient.EXPECT().CreateHost(wantRequest).Return(nil),
// 				// Post --register
// 				plugin.EXPECT().InstallHostname(hostResponseFixture.Name).Return(nil),
// 				plugin.EXPECT().InstallFile(wantAgentConfigBytes, configPathDefault, uint32(0640), 0, 0).Return(nil),
// 				plugin.EXPECT().InstallFile(gomock.Any(), wantIdentityFilePath, uint32(0640), 0, 0).Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil), // condition reporting
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil), // condition reporting
// 				// --install
// 				mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 				plugin.EXPECT().InstallCloudInit(gomock.Any()).Return(nil),
// 				plugin.EXPECT().Install(gomock.Any()).Return(nil),
// 				mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil), // condition reporting
// 				// post --install
// 				plugin.EXPECT().PowerOff().Return(nil),
// 			)
// 			Expect(cmd.Execute()).ToNot(HaveOccurred())

// 		})
// 		When("--install", func() {
// 			It("should apply cloud init, install, and mark the host as installed", func() {
// 				marshalIntoFile(fs, configFixture, configPathDefault)
// 				wantCloudInit, err := json.Marshal(registrationFixture.Config.CloudConfig)
// 				Expect(err).ToNot(HaveOccurred())
// 				wantInstall, err := json.Marshal(registrationFixture.Config.Elemental.Install)
// 				Expect(err).ToNot(HaveOccurred())
// 				cmd.SetArgs([]string{"--install"})
// 				gomock.InOrder(
// 					// Make the first get registration call fail. Expect to recover by calling again
// 					mClient.EXPECT().GetRegistration().Return(nil, errors.New("get registration test error")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.InstallationReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.InstallationFailedReason,
// 								Message:  "getting remote Registration: get registration test error",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 					// Make the cloud init apply fail. Expect to recover by getting registration and applying cloud init again
// 					plugin.EXPECT().InstallCloudInit(wantCloudInit).Return(errors.New("cloud init test failed")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.InstallationReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.CloudConfigInstallationFailedReason,
// 								Message:  "installing cloud config: cloud init test failed",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 					plugin.EXPECT().InstallCloudInit(wantCloudInit).Return(nil),
// 					// Make the install fail. Expect to recover by getting registration and installing again
// 					plugin.EXPECT().Install(wantInstall).Return(errors.New("install test fail")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.InstallationReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.InstallationFailedReason,
// 								Message:  "installing host: install test fail",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 					plugin.EXPECT().Install(wantInstall).Return(nil),
// 					// Make the patch host fail. Expect to recover by patching it again
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.InstallationReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.InstallationFailedReason,
// 								Message:  "patching host with installation successful: patch host test fail",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						if patch.Installed == nil {
// 							GinkgoT().Error("installation patch does not contain installed flag")
// 						}
// 						if !*patch.Installed {
// 							GinkgoT().Error("installation patch does not contain true installed flag")
// 						}
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.InstallationReady,
// 								Status:   corev1.ConditionTrue,
// 								Severity: clusterv1.ConditionSeverityInfo,
// 								Reason:   "",
// 								Message:  "",
// 							},
// 						), "InstallationReady True condition must be set")
// 					}),
// 					// Post --install
// 					plugin.EXPECT().PowerOff().Return(nil),
// 				)
// 				Expect(cmd.Execute()).ToNot(HaveOccurred())
// 			})
// 		})
// 		When("--reset", func() {
// 			It("should delete host, reset, and patch the host as reset", func() {
// 				marshalIntoFile(fs, configFixture, configPathDefault)
// 				wantReset, err := json.Marshal(registrationFixture.Config.Elemental.Reset)
// 				Expect(err).ToNot(HaveOccurred())
// 				cmd.SetArgs([]string{"--reset"})
// 				gomock.InOrder(
// 					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(errors.New("delete host test error")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.ResetReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.ResetFailedReason,
// 								Message:  "marking host for deletion: delete host test error",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil),
// 					// Make the first registration call fail. Expect to recover by calling again
// 					mClient.EXPECT().GetRegistration().Return(nil, errors.New("get registration test error")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.ResetReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.ResetFailedReason,
// 								Message:  "getting remote Registration: get registration test error",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil), // Always called
// 					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 					// Make the reset call fail. Expect to recover by getting registration and resetting again
// 					plugin.EXPECT().Reset(wantReset).Return(errors.New("reset test error")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.ResetReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.ResetFailedReason,
// 								Message:  "resetting host: reset test error",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil),
// 					mClient.EXPECT().GetRegistration().Return(registrationFixture, nil),
// 					plugin.EXPECT().Reset(wantReset).Return(nil),
// 					// Make the patch host fail. Expect to recover by patching it again
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.ResetReady,
// 								Status:   corev1.ConditionFalse,
// 								Severity: clusterv1.ConditionSeverityError,
// 								Reason:   infrastructurev1beta1.ResetFailedReason,
// 								Message:  "patching host with reset successful: patch host test fail",
// 							},
// 						))
// 					}),
// 					mClient.EXPECT().DeleteHost(hostResponseFixture.Name).Return(nil),
// 					mClient.EXPECT().PatchHost(gomock.Any(), hostResponseFixture.Name).Return(hostResponseFixture, nil).Do(func(patch api.HostPatchRequest, hostName string) {
// 						if patch.Reset == nil {
// 							GinkgoT().Error("reset patch does not contain reset flag")
// 						}
// 						if !*patch.Reset {
// 							GinkgoT().Error("reset patch does not contain true reset flag")
// 						}
// 						Expect(*patch.Condition).Should(Equal(
// 							clusterv1.Condition{
// 								Type:     infrastructurev1beta1.ResetReady,
// 								Status:   corev1.ConditionTrue,
// 								Severity: clusterv1.ConditionSeverityInfo,
// 								Reason:   "",
// 								Message:  "",
// 							},
// 						), "ResetReady True condition must be set")
// 					}),
// 					// Post --reset
// 					plugin.EXPECT().Reboot().Return(nil),
// 				)
// 				Expect(cmd.Execute()).ToNot(HaveOccurred())
// 			})
// 		})
// 	})
// })

// func initializeIdentity(fs vfs.FS) (identity.Identity, string) {
// 	id, err := identity.NewED25519Identity()
// 	Expect(err).ToNot(HaveOccurred())
// 	// Initialize private key on filesystem
// 	keyPath := fmt.Sprintf("%s/%s", registrationFixture.Config.Elemental.Agent.WorkDir, identity.PrivateKeyFile)
// 	idPem, err := id.Marshal()
// 	Expect(err).ToNot(HaveOccurred())
// 	Expect(vfs.MkdirAll(fs, filepath.Dir(keyPath), os.ModePerm)).Should(Succeed())
// 	Expect(fs.WriteFile(keyPath, idPem, os.ModePerm)).Should(Succeed())
// 	Expect(err).ToNot(HaveOccurred())
// 	pubKeyPem, err := id.MarshalPublic()
// 	Expect(err).ToNot(HaveOccurred())
// 	return id, string(pubKeyPem)
// }

// func marshalIntoFile(fs vfs.FS, input any, filePath string) {
// 	bytes := marshalToBytes(input)
// 	Expect(vfs.MkdirAll(fs, path.Dir(filePath), os.ModePerm)).ToNot(HaveOccurred())
// 	Expect(fs.WriteFile(filePath, bytes, os.ModePerm)).ToNot(HaveOccurred())
// }

// func marshalToBytes(input any) []byte {
// 	bytes, err := yaml.Marshal(input)
// 	Expect(err).ToNot(HaveOccurred())
// 	return bytes
// }
