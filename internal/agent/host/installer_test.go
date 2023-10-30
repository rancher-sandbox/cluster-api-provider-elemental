package host

import (
	"errors"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	gomock "go.uber.org/mock/gomock"
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

var _ = Describe("Unmanaged Installer", Label("agent", "installer", "unmanaged"), func() {
	var installer Installer
	var hostnameManager *hostname.MockManager
	var mockCtrl *gomock.Controller
	var fs vfs.FS
	var err error
	var fsCleanup func()
	BeforeEach(func() {
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl = gomock.NewController(GinkgoT())
		hostnameManager = hostname.NewMockManager(mockCtrl)
		installer = NewUnmanagedInstaller(fs, hostnameManager, testConfPath, testWorkDir)
		DeferCleanup(fsCleanup)
	})
	When("Installing", func() {
		It("should set hostname and override conf file", func() {
			marshalIntoFile(fs, config.DefaultConfig(), testConfPath) // Initialize dummy config file to be overwritten
			hostnameManager.EXPECT().SetHostname(testHostname).Return(nil)
			Expect(installer.Install(registrationFixture, testHostname)).Should(Succeed())
			compareFiles(fs, testConfPath, "_testdata/config.yaml")
		})
		It("should fail when hostname can't be set", func() {
			hostnameManager.EXPECT().SetHostname(testHostname).Return(errors.New("just a test error"))
			Expect(installer.Install(registrationFixture, testHostname)).ShouldNot(Succeed())
		})
		It("should write install config to file", func() {
			testInstallPath := fmt.Sprintf("%s/install.yaml", testWorkDir)
			marshalIntoFile(fs, []byte("to be overwritten"), testInstallPath) // Initialize dummy install file to be overwritten
			hostnameManager.EXPECT().SetHostname(testHostname).Return(nil)
			Expect(installer.Install(registrationFixture, testHostname)).Should(Succeed())
			compareFiles(fs, testInstallPath, "_testdata/install.yaml")
		})
		It("should write cloud-init config to file", func() {
			testCloudInitPath := fmt.Sprintf("%s/cloud-init.yaml", testWorkDir)
			marshalIntoFile(fs, []byte("to be overwritten"), testCloudInitPath) // Initialize dummy cloud-init file to be overwritten
			hostnameManager.EXPECT().SetHostname(testHostname).Return(nil)
			Expect(installer.Install(registrationFixture, testHostname)).Should(Succeed())
			compareFiles(fs, testCloudInitPath, "_testdata/cloud-init.yaml")
		})
	})
	When("Triggering Reset", func() {
		It("should write reset sentinel file", func() {
			Expect(installer.TriggerReset()).Should(Succeed())
			_, err := fs.Stat(fmt.Sprintf("%s/%s", testWorkDir, sentinelFileResetNeeded))
			Expect(err).ToNot(HaveOccurred())
		})
	})
	When("Resetting", func() {
		It("should fail if reset sentinel file exists", func() {
			Expect(installer.TriggerReset()).Should(Succeed()) // Trigger reset to create sentinel file
			Expect(installer.Reset(registrationFixture)).ShouldNot(Succeed())
		})
		It("should succeed if reset sentinel file was deleted", func() {
			Expect(installer.Reset(registrationFixture)).Should(Succeed())
		})
		It("should write reset config to file", func() {
			testResetPath := fmt.Sprintf("%s/reset.yaml", testWorkDir)
			marshalIntoFile(fs, []byte("to be overwritten"), testResetPath) // Initialize dummy reset file to be overwritten
			Expect(installer.Reset(registrationFixture)).Should(Succeed())
			compareFiles(fs, testResetPath, "_testdata/reset.yaml")
		})
	})
})

var _ = Describe("Elemental Installer", Label("agent", "installer", "elemental"), func() {
	elementalRegistration := registrationFixture
	elementalRegistration.Config.Elemental.Install = map[string]runtime.RawExtension{
		"firmware":         {Raw: []byte(`"test firmware"`)},
		"device":           {Raw: []byte(`"test device"`)},
		"noFormat":         {Raw: []byte("true")},
		"configUrls":       {Raw: []byte(`["test config url 1", "test config url 2"]`)},
		"iso":              {Raw: []byte(`"test iso"`)},
		"systemUri":        {Raw: []byte(`"test system uri"`)},
		"debug":            {Raw: []byte("true")},
		"tty":              {Raw: []byte(`"test tty"`)},
		"poweroff":         {Raw: []byte("true")},
		"reboot":           {Raw: []byte("true")},
		"ejectCd":          {Raw: []byte("true")},
		"disableBootEntry": {Raw: []byte("true")},
		"configDir":        {Raw: []byte(`"test config dir"`)},
	}
	wantInstall := elementalcli.Install{
		Firmware:         "test firmware",
		Device:           "test device",
		NoFormat:         true,
		ConfigURLs:       []string{"test config url 1", "test config url 2", "/tmp/cloud-init.yaml", "/tmp/host-config.yaml"},
		ISO:              "test iso",
		SystemURI:        "test system uri",
		Debug:            true,
		TTY:              "test tty",
		PowerOff:         true,
		Reboot:           true,
		EjectCD:          true,
		DisableBootEntry: true,
		ConfigDir:        "test config dir",
	}
	elementalRegistration.Config.Elemental.Reset = map[string]runtime.RawExtension{
		"enabled":         {Raw: []byte("true")},
		"resetPersistent": {Raw: []byte("true")},
		"resetOem":        {Raw: []byte("true")},
		"configUrls":      {Raw: []byte(`["test config url 1", "test config url 2"]`)},
		"systemUri":       {Raw: []byte(`"test system uri"`)},
		"debug":           {Raw: []byte("true")},
		"poweroff":        {Raw: []byte("true")},
		"reboot":          {Raw: []byte("true")},
	}
	wantReset := elementalcli.Reset{
		Enabled:         true,
		ResetPersistent: true,
		ResetOEM:        true,
		ConfigURLs:      []string{"test config url 1", "test config url 2"},
		SystemURI:       "test system uri",
		Debug:           true,
		PowerOff:        true,
		Reboot:          true,
	}
	var installer Installer
	var hostnameManager *hostname.MockManager
	var cliRunner *elementalcli.MockRunner
	var cmdRunner *utils.MockCommandRunner
	var mockCtrl *gomock.Controller
	var fs vfs.FS
	var err error
	var fsCleanup func()
	BeforeEach(func() {
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl = gomock.NewController(GinkgoT())
		hostnameManager = hostname.NewMockManager(mockCtrl)
		cliRunner = elementalcli.NewMockRunner(mockCtrl)
		cmdRunner = utils.NewMockCommandRunner(mockCtrl)
		installer = &ElementalInstaller{
			fs:              fs,
			cliRunner:       cliRunner,
			cmdRunner:       cmdRunner,
			hostnameManager: hostnameManager,
			workDir:         testWorkDir,
			configPath:      testConfPath,
		}
		DeferCleanup(fsCleanup)
	})
	When("Installing", func() {
		It("should call elemental install", func() {
			hostnameManager.EXPECT().SetHostname(testHostname).Return(nil)
			cliRunner.EXPECT().Install(wantInstall).Return(nil)
			Expect(installer.Install(elementalRegistration, testHostname)).Should(Succeed())
		})
		It("should write temporary host config", func() {
			hostnameManager.EXPECT().SetHostname(testHostname).Return(nil)
			cliRunner.EXPECT().Install(wantInstall).Return(nil)
			Expect(installer.Install(elementalRegistration, testHostname)).Should(Succeed())
			compareFiles(fs, "/tmp/host-config.yaml", "_testdata/host-config.yaml")
		})
		It("should write temporary cloud init", func() {
			hostnameManager.EXPECT().SetHostname(testHostname).Return(nil)
			cliRunner.EXPECT().Install(wantInstall).Return(nil)
			Expect(installer.Install(elementalRegistration, testHostname)).Should(Succeed())
			compareFiles(fs, "/tmp/cloud-init.yaml", "_testdata/cloud-init.yaml")
		})
	})
	When("Triggering reset", func() {
		It("should set next grub entry and schedule reboot", func() {
			cmdRunner.EXPECT().RunCommand("grub2-editenv /oem/grubenv set next_entry=recovery").Return(nil)
			cmdRunner.EXPECT().RunCommand("shutdown -r +1").Return(nil)
			Expect(installer.TriggerReset()).Should(Succeed())
		})
		It("should write reset cloud config", func() {
			cmdRunner.EXPECT().RunCommand(gomock.Any()).AnyTimes().Return(nil)
			Expect(installer.TriggerReset()).Should(Succeed())
			compareFiles(fs, "/oem/reset-cloud-config.yaml", "_testdata/reset-cloud-config.yaml")
		})
	})
	When("Resetting", func() {
		It("should call elemental reset", func() {
			cliRunner.EXPECT().Reset(wantReset).Return(nil)
			Expect(installer.Reset(elementalRegistration)).Should(Succeed())
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

func compareFiles(fs vfs.FS, got string, want string) {
	gotFile, err := fs.ReadFile(got)
	Expect(err).ToNot(HaveOccurred())
	wantFile, err := os.ReadFile(want)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gotFile)).To(Equal(string(wantFile)))
}
