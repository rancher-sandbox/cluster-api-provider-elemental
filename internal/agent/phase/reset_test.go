package phase

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	gomock "go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("reset handler", Label("cli", "phases", "reset"), func() {
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var plugin *osplugin.MockPlugin
	var handler ResetHandler
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
		handler = NewResetHandler(agentContext)
	})
	When("triggering reset", func() {
		It("should trigger reset", func() {
			gomock.InOrder(
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1.PhaseTriggeringReset)}, HostResponseFixture.Name),
				plugin.EXPECT().TriggerReset().Return(nil),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1.ResetReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityInfo,
							Reason:   infrastructurev1.WaitingForResetReason,
							Message:  "Reset was triggered successfully. Waiting for host to reset.",
						},
					))
				}),
			)
			err := handler.TriggerReset()
			Expect(err).ToNot(HaveOccurred())
		})
		It("should fail on trigger reset error", func() {
			wantErr := errors.New("test trigger reset error")

			gomock.InOrder(
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1.PhaseTriggeringReset)}, HostResponseFixture.Name),
				plugin.EXPECT().TriggerReset().Return(wantErr),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1.ResetReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1.ResetFailedReason,
							Message:  "triggering reset: " + wantErr.Error(),
						},
					))
				}),
			)
			err := handler.TriggerReset()
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})
	})
	When("resetting", func() {
		It("should delete host, reset, and patch the host as reset", func() {
			wantReset, err := json.Marshal(RegistrationFixture.Config.Elemental.Reset)
			Expect(err).ToNot(HaveOccurred())
			gomock.InOrder(
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1.PhaseResetting)}, HostResponseFixture.Name),
				mClient.EXPECT().DeleteHost(HostResponseFixture.Name).Return(errors.New("delete host test error")),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1.ResetReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1.ResetFailedReason,
							Message:  "marking host for deletion: delete host test error",
						},
					))
				}),
				mClient.EXPECT().DeleteHost(HostResponseFixture.Name).Return(nil),
				// Make the first registration call fail. Expect to recover by calling again
				mClient.EXPECT().GetRegistration().Return(nil, errors.New("get registration test error")),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1.ResetReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1.ResetFailedReason,
							Message:  "getting remote Registration: get registration test error",
						},
					))
				}),
				mClient.EXPECT().DeleteHost(HostResponseFixture.Name).Return(nil), // Always called
				mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
				// Make the reset call fail. Expect to recover by getting registration and resetting again
				plugin.EXPECT().Reset(wantReset).Return(errors.New("reset test error")),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1.ResetReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1.ResetFailedReason,
							Message:  "resetting host: reset test error",
						},
					))
				}),
				mClient.EXPECT().DeleteHost(HostResponseFixture.Name).Return(nil),
				mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
				plugin.EXPECT().Reset(wantReset).Return(nil),
				// Make the patch host fail. Expect to recover by patching it again
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1.ResetReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1.ResetFailedReason,
							Message:  "patching host with reset successful: patch host test fail",
						},
					))
				}),
				mClient.EXPECT().DeleteHost(HostResponseFixture.Name).Return(nil),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(&HostResponseFixture, nil).Do(func(patch api.HostPatchRequest, _ string) {
					if patch.Reset == nil {
						GinkgoT().Error("reset patch does not contain reset flag")
					}
					if !*patch.Reset {
						GinkgoT().Error("reset patch does not contain true reset flag")
					}
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1.ResetReady,
							Status:   corev1.ConditionTrue,
							Severity: clusterv1.ConditionSeverityInfo,
							Reason:   "",
							Message:  "",
						},
					), "ResetReady True condition must be set")
				}),
			)

			handler.Reset()
		})
	})
})
