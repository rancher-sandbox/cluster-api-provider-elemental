package installer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	gomock "go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/runtime"
)

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
	}
	var installer Installer
	var hostManager *host.MockManager
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
		hostManager = host.NewMockManager(mockCtrl)
		cliRunner = elementalcli.NewMockRunner(mockCtrl)
		cmdRunner = utils.NewMockCommandRunner(mockCtrl)
		installer = &ElementalInstaller{
			fs:          fs,
			cliRunner:   cliRunner,
			cmdRunner:   cmdRunner,
			hostManager: hostManager,
			workDir:     testWorkDir,
			configPath:  testConfPath,
		}
		DeferCleanup(fsCleanup)
	})
	When("Installing", func() {
		It("should call elemental install", func() {
			hostManager.EXPECT().SetHostname(testHostname).Return(nil)
			cliRunner.EXPECT().Install(wantInstall).Return(nil)
			Expect(installer.Install(elementalRegistration, testHostname)).Should(Succeed())
		})
		It("should write temporary host config", func() {
			hostManager.EXPECT().SetHostname(testHostname).Return(nil)
			cliRunner.EXPECT().Install(wantInstall).Return(nil)
			Expect(installer.Install(elementalRegistration, testHostname)).Should(Succeed())
			compareFiles(fs, "/tmp/host-config.yaml", "_testdata/host-config.yaml")
		})
		It("should write temporary cloud init", func() {
			hostManager.EXPECT().SetHostname(testHostname).Return(nil)
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
