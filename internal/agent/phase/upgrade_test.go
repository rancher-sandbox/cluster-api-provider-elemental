package phase

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	gomock "go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("upgrade handler", Label("cli", "phases", "upgrade"), func() {
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var plugin *osplugin.MockPlugin
	var handler OSVersionHandler
	var agentContext context.AgentContext

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		plugin = osplugin.NewMockPlugin(mockCtrl)
		agentContext = context.AgentContext{
			Plugin:     plugin,
			Client:     mClient,
			Config:     ConfigFixture,
			ConfigPath: ConfigPathFixture,
			Hostname:   HostResponseFixture.Name,
		}
		handler = NewOSVersionHandler(agentContext)
	})

	It("should reconcile os version and reboot", func() {
		wantOSVersion, err := json.Marshal(OSVersionManagementFixture)
		Expect(err).ToNot(HaveOccurred())
		gomock.InOrder(
			// Expect plugin to reconcile OS Version and ask for machine reboot
			plugin.EXPECT().ReconcileOSVersion(wantOSVersion).Return(true, nil),
			// Expect phase update
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Phase).Should(Equal(infrastructurev1.PhaseOSVersionReconcile))
			}),
			// Expect condition update
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1.OSVersionReady,
						Status:   corev1.ConditionFalse,
						Severity: infrastructurev1.WaitingForPostReconcileRebootReasonSeverity,
						Reason:   infrastructurev1.WaitingForPostReconcileRebootReason,
						Message:  "Waiting for Host to reboot after OS Version has been reconciled.",
					},
				))
			}),
		)
		inPlaceUpdate := false
		postAction, err := handler.Reconcile(OSVersionManagementFixture, inPlaceUpdate)
		Expect(err).ToNot(HaveOccurred())
		Expect(postAction.PowerOff).Should(BeFalse(), "Machine should not shut down")
		Expect(postAction.Reboot).Should(BeTrue(), "Machine should reboot")
	})
	It("should mark os version as successfully reconciled", func() {
		wantOSVersion, err := json.Marshal(OSVersionManagementFixture)
		Expect(err).ToNot(HaveOccurred())
		gomock.InOrder(
			// Expect plugin to reconcile OS Version
			plugin.EXPECT().ReconcileOSVersion(wantOSVersion).Return(false, nil),
			// Expect condition update
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1.OSVersionReady,
						Status:   corev1.ConditionTrue,
						Severity: clusterv1.ConditionSeverityInfo,
						Reason:   "",
						Message:  "",
					},
				))
			}),
		)
		inPlaceUpdate := false
		postAction, err := handler.Reconcile(OSVersionManagementFixture, inPlaceUpdate)
		Expect(err).ToNot(HaveOccurred())
		Expect(postAction.PowerOff).Should(BeFalse(), "Machine should not shut down")
		Expect(postAction.Reboot).Should(BeFalse(), "Machine should not reboot")
	})
	It("should mark in place update as done", func() {
		wantOSVersion, err := json.Marshal(OSVersionManagementFixture)
		Expect(err).ToNot(HaveOccurred())
		gomock.InOrder(
			// Expect plugin to reconcile OS Version
			plugin.EXPECT().ReconcileOSVersion(wantOSVersion).Return(false, nil),
			// Expect condition update
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1.OSVersionReady,
						Status:   corev1.ConditionTrue,
						Severity: clusterv1.ConditionSeverityInfo,
						Reason:   "",
						Message:  "",
					},
				))
			}),
			// Expect InPlaceUpdateDone update
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.InPlaceUpdate).Should(Equal(infrastructurev1.InPlaceUpdateDone))
			}),
		)
		inPlaceUpdate := true
		postAction, err := handler.Reconcile(OSVersionManagementFixture, inPlaceUpdate)
		Expect(err).ToNot(HaveOccurred())
		Expect(postAction.PowerOff).Should(BeFalse(), "Machine should not shut down")
		Expect(postAction.Reboot).Should(BeFalse(), "Machine should not reboot")
	})
})
