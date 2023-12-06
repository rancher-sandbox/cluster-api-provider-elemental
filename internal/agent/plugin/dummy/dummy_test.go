package main

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	"go.uber.org/mock/gomock"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Dummy Plugin Suite")
}

var _ = Describe("Dummy Plugin", Label("agent", "plugin", "dummy"), func() {
	var plugin osplugin.Plugin
	var hostManager *host.MockManager
	var fs vfs.FS
	var err error
	var fsCleanup func()

	pluginContext := osplugin.PluginContext{
		WorkDir:    "/just/a/test/workdir",
		ConfigPath: "/test/config/dir/test-config.yaml",
		Debug:      true,
	}

	BeforeEach(func() {
		Expect(err).ToNot(HaveOccurred())
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl := gomock.NewController(GinkgoT())
		hostManager = host.NewMockManager(mockCtrl)
		plugin = &DummyPlugin{
			hostManager: hostManager,
			fs:          fs,
		}
		Expect(plugin.Init(pluginContext)).Should(Succeed())
		DeferCleanup(fsCleanup)
	})
	It("should apply cloud-init by dumping info to file", func() {
		cloudInit := []byte(`{"users":[{"name":"root","passwd":"root"}]}`)
		Expect(plugin.InstallCloudInit(cloudInit)).Should(Succeed())
		wantPath := fmt.Sprintf("%s/%s", pluginContext.WorkDir, cloudInitFile)
		compareFiles(fs, wantPath, "_testdata/cloud-init.yaml")
	})
	It("should return current hostname", func() {
		wantHostname := "just a test hostname"
		hostManager.EXPECT().GetCurrentHostname().Return(wantHostname, nil)
		gotHostname, err := plugin.GetHostname()
		Expect(err).ToNot(HaveOccurred())
		Expect(gotHostname).To(Equal(wantHostname))
	})
	It("should set the hostname", func() {
		wantHostname := "just a test hostname to set"
		hostManager.EXPECT().SetHostname(wantHostname).Return(nil)
		Expect(plugin.InstallHostname(wantHostname)).Should(Succeed())
	})
	It("should write file", func() {
		content := []byte("Just a test file\n")
		wantPath := "/any/location/should/be/fine"
		Expect(plugin.InstallFile(content, wantPath, 0640, 0, 0)).Should(Succeed())
		compareFiles(fs, wantPath, "_testdata/persisted.txt")
	})
	It("should install by dumping info to file", func() {
		input := []byte(`{"foo":{"bar":{"foobar":"barfoo"}}}`)
		Expect(plugin.Install(input)).Should(Succeed())
		wantPath := fmt.Sprintf("%s/%s", pluginContext.WorkDir, installFile)
		compareFiles(fs, wantPath, "_testdata/install.yaml")
	})
	It("should trigger reset by creating file", func() {
		Expect(plugin.TriggerReset()).Should(Succeed())
		wantPath := fmt.Sprintf("%s/%s", pluginContext.WorkDir, sentinelFileResetNeeded)
		compareFiles(fs, wantPath, "_testdata/needs.reset")
	})
	It("should fail to reset if needs.reset file is still present", func() {
		needsResetPath := fmt.Sprintf("%s/%s", pluginContext.WorkDir, sentinelFileResetNeeded)
		Expect(fs.WriteFile(needsResetPath, []byte("anything"), os.ModePerm)).Should(Succeed())
		Expect(plugin.Reset([]byte(""))).ShouldNot(Succeed())
	})
	It("should reset by dumpint info to file", func() {
		input := []byte(`{"foo":{"bar":{"foobar":"barfoo"}}}`)
		Expect(plugin.Reset(input)).Should(Succeed())
		wantPath := fmt.Sprintf("%s/%s", pluginContext.WorkDir, resetFile)
		compareFiles(fs, wantPath, "_testdata/reset.yaml")
	})
	It("should poweroff", func() {
		hostManager.EXPECT().PowerOff().Return(nil)
		Expect(plugin.PowerOff()).Should(Succeed())
	})
	It("should reboot", func() {
		hostManager.EXPECT().Reboot().Return(nil)
		Expect(plugin.Reboot()).Should(Succeed())
	})
	It("should apply cloud-init bootstrap", func() {
		wantInput := "foo\n"
		Expect(plugin.Bootstrap("cloud-config", []byte(wantInput))).Should(Succeed())
		compareFiles(fs, bootstrapCloudInitPath, "_testdata/bootstrap.config")
	})
	It("should apply ignition bootstrap", func() {
		wantInput := "foo\n"
		Expect(plugin.Bootstrap("ignition", []byte(wantInput))).Should(Succeed())
		compareFiles(fs, bootstrapIgnitionPath, "_testdata/bootstrap.config")
	})
	It("should fail on unknown bootstrap format", func() {
		Expect(plugin.Bootstrap("uknown", []byte(""))).ShouldNot(Succeed())
	})
})

func compareFiles(fs vfs.FS, got string, want string) {
	gotFile, err := fs.ReadFile(got)
	Expect(err).ToNot(HaveOccurred())
	wantFile, err := os.ReadFile(want)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gotFile)).To(Equal(string(wantFile)))
}
