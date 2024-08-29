package phase

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	gomock "go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("install handler", Label("cli", "phases", "install"), func() {
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var plugin *osplugin.MockPlugin
	var id *identity.MockIdentity
	var handler InstallHandler
	var agentContext context.AgentContext

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		plugin = osplugin.NewMockPlugin(mockCtrl)
		id = identity.NewMockIdentity(mockCtrl)
		agentContext = context.AgentContext{
			Identity:   id,
			Plugin:     plugin,
			Client:     mClient,
			Config:     ConfigFixture,
			ConfigPath: ConfigPathFixture,
			Hostname:   HostResponseFixture.Name,
		}
		handler = NewInstallHandler(agentContext)
	})
	It("should apply cloud init, install, and mark the host as installed", func() {
		wantCloudInit, err := json.Marshal(RegistrationFixture.Config.CloudConfig)
		Expect(err).ToNot(HaveOccurred())
		wantInstall, err := json.Marshal(RegistrationFixture.Config.Elemental.Install)
		Expect(err).ToNot(HaveOccurred())
		gomock.InOrder(
			// Expect phase to be updated
			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1beta1.PhaseInstalling)}, HostResponseFixture.Name),
			// Make the first get registration call fail. Expect to recover by calling again
			mClient.EXPECT().GetRegistration().Return(nil, errors.New("get registration test error")),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1beta1.InstallationReady,
						Status:   corev1.ConditionFalse,
						Severity: clusterv1.ConditionSeverityError,
						Reason:   infrastructurev1beta1.InstallationFailedReason,
						Message:  "getting remote Registration: get registration test error",
					},
				))
			}),
			mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
			// Make the cloud init apply fail. Expect to recover by getting registration and applying cloud init again
			plugin.EXPECT().InstallCloudInit(wantCloudInit).Return(errors.New("cloud init test failed")),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1beta1.InstallationReady,
						Status:   corev1.ConditionFalse,
						Severity: clusterv1.ConditionSeverityError,
						Reason:   infrastructurev1beta1.CloudConfigInstallationFailedReason,
						Message:  "installing cloud config: cloud init test failed",
					},
				))
			}),
			mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
			plugin.EXPECT().InstallCloudInit(wantCloudInit).Return(nil),
			// Make the install fail. Expect to recover by getting registration and installing again
			plugin.EXPECT().Install(wantInstall).Return(errors.New("install test fail")),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1beta1.InstallationReady,
						Status:   corev1.ConditionFalse,
						Severity: clusterv1.ConditionSeverityError,
						Reason:   infrastructurev1beta1.InstallationFailedReason,
						Message:  "installing host: install test fail",
					},
				))
			}),
			mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
			plugin.EXPECT().Install(wantInstall).Return(nil),
			// Make the patch host fail. Expect to recover by patching it again
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, errors.New("patch host test fail")),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1beta1.InstallationReady,
						Status:   corev1.ConditionFalse,
						Severity: clusterv1.ConditionSeverityError,
						Reason:   infrastructurev1beta1.InstallationFailedReason,
						Message:  "patching host with installation successful: patch host test fail",
					},
				))
			}),
			mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(&HostResponseFixture, nil).Do(func(patch api.HostPatchRequest, _ string) {
				if patch.Installed == nil {
					GinkgoT().Error("installation patch does not contain installed flag")
				}
				if !*patch.Installed {
					GinkgoT().Error("installation patch does not contain true installed flag")
				}
				Expect(*patch.Condition).Should(Equal(
					clusterv1.Condition{
						Type:     infrastructurev1beta1.InstallationReady,
						Status:   corev1.ConditionTrue,
						Severity: clusterv1.ConditionSeverityInfo,
						Reason:   "",
						Message:  "",
					},
				), "InstallationReady True condition must be set")
			}),
		)

		handler.Install()
	})
})
