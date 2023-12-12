package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	"go.uber.org/mock/gomock"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Elemental Plugin Suite")
}

var _ = Describe("Elemental Plugin", Label("agent", "plugin", "elemental"), func() {
	var plugin osplugin.Plugin
	var hostManager *host.MockManager
	var cmdRunner *utils.MockCommandRunner
	var cliRunner *elementalcli.MockRunner
	var fs vfs.FS
	var err error
	var fsCleanup func()

	pluginContext := osplugin.PluginContext{
		WorkDir:    "/just/a/test/workdir",
		ConfigPath: "/test/config/dir/test-config.yaml",
		Debug:      true,
	}
	install := elementalcli.Install{
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
	reset := elementalcli.Reset{
		Enabled:         true,
		ResetPersistent: true,
		ResetOEM:        true,
		ConfigURLs:      []string{"test config url 1", "test config url 2"},
		SystemURI:       "test system uri",
		Debug:           true,
	}

	BeforeEach(func() {
		Expect(err).ToNot(HaveOccurred())
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl := gomock.NewController(GinkgoT())
		hostManager = host.NewMockManager(mockCtrl)
		cmdRunner = utils.NewMockCommandRunner(mockCtrl)
		cliRunner = elementalcli.NewMockRunner(mockCtrl)
		plugin = &ElementalPlugin{
			cliRunner:   cliRunner,
			cmdRunner:   cmdRunner,
			hostManager: hostManager,
			fs:          fs,
		}
		Expect(plugin.Init(pluginContext)).Should(Succeed())
		DeferCleanup(fsCleanup)
	})
	It("should apply cloud-init by dumping info to file", func() {
		cloudInit := []byte(`{"users":[{"name":"root","passwd":"root"}]}`)
		Expect(plugin.InstallCloudInit(cloudInit)).Should(Succeed())
		compareFiles(fs, cloudConfigInitPath, "_testdata/set-cloud-config.yaml")
	})
	It("should return current hostname", func() {
		wantHostname := "just a test hostname"
		hostManager.EXPECT().GetCurrentHostname().Return(wantHostname, nil)
		gotHostname, err := plugin.GetHostname()
		Expect(err).ToNot(HaveOccurred())
		Expect(gotHostname).To(Equal(wantHostname))
	})
	It("should write a set-hostname.yaml file", func() {
		wantHostname := "just a test hostname to set"
		Expect(plugin.InstallHostname(wantHostname)).Should(Succeed())
		compareFiles(fs, hostnameInitPath, "_testdata/set-hostname.yaml")
	})
	It("should write file", func() {
		content := []byte("Just a test file")
		wantPath := "/any/location/should/be.fine"
		Expect(plugin.InstallFile(content, wantPath, 0640, 0, 0)).Should(Succeed())
		wantSetPath := fmt.Sprintf("%s/set-be-fine.yaml", cloudConfigDir)
		compareFiles(fs, wantSetPath, "_testdata/set-file.yaml")
	})
	It("should only install when running in live mode", func() {
		Expect(plugin.Install([]byte("{}"))).Should(Succeed())
		cliRunner.EXPECT().Install(gomock.Any()).Times(0)
	})
	It("should install invoking elemental install", func() {
		Expect(vfs.MkdirAll(fs, "/run/cos", os.ModePerm)).Should(Succeed())
		Expect(fs.WriteFile("/run/cos/live_mode", []byte{}, os.ModePerm)).Should(Succeed())
		installJSON, err := json.Marshal(install)
		installWithSetFiles := install
		installWithSetFiles.ConfigURLs = append(installWithSetFiles.ConfigURLs, hostnameInitPath, identityInitPath, agentConfigInitPath, cloudConfigInitPath)
		Expect(err).ToNot(HaveOccurred())
		cliRunner.EXPECT().Install(installWithSetFiles).Return(nil)
		Expect(plugin.Install(installJSON)).Should(Succeed())
	})
	It("should trigger reset by creating reset cloud init-file and scheduling recovery reboot", func() {
		gomock.InOrder(
			cmdRunner.EXPECT().RunCommand("grub2-editenv /oem/grubenv set next_entry=recovery").Return(nil),
			cmdRunner.EXPECT().RunCommand("shutdown -r +1").Return(nil),
		)
		Expect(plugin.TriggerReset()).Should(Succeed())
		compareFiles(fs, resetCloudConfigPath, "_testdata/reset-cloud-config.yaml")
	})
	It("should reset by invoking elemental reset and restoring agent config", func() {
		gomock.InOrder(
			cmdRunner.EXPECT().RunCommand("cp /test/config/dir/test-config.yaml /tmp/elemental-agent-config.yaml").Return(nil),
			cliRunner.EXPECT().Reset(reset).Return(nil),
			cmdRunner.EXPECT().RunCommand("mount /oem").Return(nil),
			cmdRunner.EXPECT().RunCommand("mv /tmp/elemental-agent-config.yaml /test/config/dir/test-config.yaml").Return(nil),
		)
		resetJSON, err := json.Marshal(reset)
		Expect(err).ToNot(HaveOccurred())
		Expect(plugin.Reset(resetJSON)).Should(Succeed())
		agentConfigDir, err := fs.Stat("/test/config/dir")
		Expect(err).ToNot(HaveOccurred(), "Agent config dir must exist in order to mv file back to it")
		Expect(agentConfigDir.IsDir()).To(BeTrue())
	})
	It("should poweroff", func() {
		hostManager.EXPECT().PowerOff().Return(nil)
		Expect(plugin.PowerOff()).Should(Succeed())
	})
	It("should reboot", func() {
		hostManager.EXPECT().Reboot().Return(nil)
		Expect(plugin.Reboot()).Should(Succeed())
	})
	It("should bootstrap cloud-init", func() {
		capiBootstrap, err := os.ReadFile("_testdata/capi-bootstrap.yaml")
		Expect(err).ToNot(HaveOccurred())
		Expect(plugin.Bootstrap("cloud-config", capiBootstrap)).Should(Succeed())
		compareFiles(fs, bootstrapPath, "_testdata/capi-yipified.yaml")
	})
	It("should fail bootstrap on unsupported format", func() {
		Expect(plugin.Bootstrap("ignition", []byte(""))).ShouldNot(Succeed())
	})
})

func compareFiles(fs vfs.FS, got string, want string) {
	gotFile, err := fs.ReadFile(got)
	Expect(err).ToNot(HaveOccurred())
	wantFile, err := os.ReadFile(want)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gotFile)).To(Equal(string(wantFile)))
}
