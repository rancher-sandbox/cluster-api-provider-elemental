package phase

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
	gomock "go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("bootstrap handler", Label("cli", "phases", "bootstrap"), func() {
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var plugin *osplugin.MockPlugin
	var fs vfs.FS
	var fsCleanup func()
	var err error
	var handler BootstrapHandler
	var agentContext context.AgentContext

	bootstrapResponse := api.BootstrapResponse{
		Format: "foo",
		Config: "bar",
	}

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		plugin = osplugin.NewMockPlugin(mockCtrl)
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		agentContext = context.AgentContext{
			Plugin:     plugin,
			Client:     mClient,
			Config:     ConfigFixture,
			ConfigPath: ConfigPathFixture,
			Hostname:   HostResponseFixture.Name,
		}
		handler = &bootstrapHandler{
			agentContext: agentContext,
			fs:           fs,
		}
		DeferCleanup(fsCleanup)
	})
	It("should bootstrap when bootstrap sentinel file missing", func() {
		gomock.InOrder(
			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1.PhaseBootstrapping)}, HostResponseFixture.Name),
			mClient.EXPECT().GetBootstrap(HostResponseFixture.Name).Return(&bootstrapResponse, nil),
			plugin.EXPECT().Bootstrap(bootstrapResponse.Format, []byte(bootstrapResponse.Config)).Return(nil),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1.BootstrapReady,
						Status:   corev1.ConditionFalse,
						Severity: infrastructurev1.WaitingForBootstrapReasonSeverity,
						Reason:   infrastructurev1.WaitingForBootstrapReason,
						Message:  "Waiting for bootstrap to be executed",
					},
				))
			}),
		)

		post, err := handler.Bootstrap()
		Expect(err).ToNot(HaveOccurred())
		Expect(post).To(Equal(infrastructurev1.PostAction{Reboot: true}), "System must reboot to apply bootstrap config")
	})
	It("should patch the host as bootstrapped when sentinel file is present", func() {
		// Mark the system as bootstrapped. This path is part of the CAPI contract: https://cluster-api.sigs.k8s.io/developer/providers/bootstrap.html#sentinel-file
		Expect(vfs.MkdirAll(fs, "/run/cluster-api", os.ModePerm)).Should(Succeed())
		Expect(fs.WriteFile("/run/cluster-api/bootstrap-success.complete", []byte("anything"), os.ModePerm)).Should(Succeed())
		gomock.InOrder(
			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1.PhaseBootstrapping)}, HostResponseFixture.Name),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(&HostResponseFixture, nil).Do(func(patch api.HostPatchRequest, _ string) {
				if patch.Bootstrapped == nil {
					GinkgoT().Error("bootstrapped patch does not contain bootstrapped flag")
				}
				if !*patch.Bootstrapped {
					GinkgoT().Error("bootstrapped patch does not contain true bootstrapped flag")
				}
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1.BootstrapReady,
						Status:   corev1.ConditionTrue,
						Severity: clusterv1.ConditionSeverityInfo,
						Reason:   "",
						Message:  "",
					},
				))
			}),
		)
		post, err := handler.Bootstrap()
		Expect(err).ToNot(HaveOccurred())
		Expect(post).To(Equal(infrastructurev1.PostAction{}))
	})
	It("should fail on bootstrap error", func() {
		wantErr := errors.New("test bootstrap error")

		gomock.InOrder(
			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1.PhaseBootstrapping)}, HostResponseFixture.Name),
			mClient.EXPECT().GetBootstrap(HostResponseFixture.Name).Return(&bootstrapResponse, nil),
			plugin.EXPECT().Bootstrap(bootstrapResponse.Format, []byte(bootstrapResponse.Config)).Return(wantErr),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1.BootstrapReady,
						Status:   corev1.ConditionFalse,
						Severity: clusterv1.ConditionSeverityError,
						Reason:   infrastructurev1.BootstrapFailedReason,
						Message:  "applying bootstrap config: " + wantErr.Error(),
					},
				))
			}),
		)

		post, err := handler.Bootstrap()
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, wantErr)).To(BeTrue())
		Expect(post).To(Equal(infrastructurev1.PostAction{}))
	})
})
