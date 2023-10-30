package installer

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	gomock "go.uber.org/mock/gomock"
)

var _ = Describe("Unmanaged Installer", Label("agent", "installer", "unmanaged"), func() {
	var installer Installer
	var hostManager *host.MockManager
	var mockCtrl *gomock.Controller
	var fs vfs.FS
	var err error
	var fsCleanup func()
	BeforeEach(func() {
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		mockCtrl = gomock.NewController(GinkgoT())
		hostManager = host.NewMockManager(mockCtrl)
		installer = NewUnmanagedInstaller(fs, hostManager, testConfPath, testWorkDir)
		DeferCleanup(fsCleanup)
	})
	When("Installing", func() {
		It("should set hostname and override conf file", func() {
			marshalIntoFile(fs, config.DefaultConfig(), testConfPath) // Initialize dummy config file to be overwritten
			hostManager.EXPECT().SetHostname(testHostname).Return(nil)
			Expect(installer.Install(registrationFixture, testHostname)).Should(Succeed())
			compareFiles(fs, testConfPath, "_testdata/config.yaml")
		})
		It("should fail when hostname can't be set", func() {
			hostManager.EXPECT().SetHostname(testHostname).Return(errors.New("just a test error"))
			Expect(installer.Install(registrationFixture, testHostname)).ShouldNot(Succeed())
		})
		It("should write install config to file", func() {
			testInstallPath := fmt.Sprintf("%s/install.yaml", testWorkDir)
			marshalIntoFile(fs, []byte("to be overwritten"), testInstallPath) // Initialize dummy install file to be overwritten
			hostManager.EXPECT().SetHostname(testHostname).Return(nil)
			Expect(installer.Install(registrationFixture, testHostname)).Should(Succeed())
			compareFiles(fs, testInstallPath, "_testdata/install.yaml")
		})
		It("should write cloud-init config to file", func() {
			testCloudInitPath := fmt.Sprintf("%s/cloud-init.yaml", testWorkDir)
			marshalIntoFile(fs, []byte("to be overwritten"), testCloudInitPath) // Initialize dummy cloud-init file to be overwritten
			hostManager.EXPECT().SetHostname(testHostname).Return(nil)
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
