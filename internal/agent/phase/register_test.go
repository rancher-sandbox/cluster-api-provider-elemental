package phase

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	gomock "go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("registration handler", Label("cli", "phases", "registration"), func() {
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var plugin *osplugin.MockPlugin
	var id *identity.MockIdentity
	var handler RegistrationHandler
	var agentContext *context.AgentContext

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		plugin = osplugin.NewMockPlugin(mockCtrl)
		id = identity.NewMockIdentity(mockCtrl)
		agentContext = &context.AgentContext{
			Identity:   id,
			Plugin:     plugin,
			Client:     mClient,
			Config:     ConfigFixture,
			ConfigPath: ConfigPathFixture,
			Hostname:   HostResponseFixture.Name,
		}
		handler = NewRegistrationHandler(agentContext)
	})
	When("registering", func() {
		wantPubKey := []byte("just a test pubkey")

		wantRequest := api.HostCreateRequest{
			Name:        HostResponseFixture.Name,
			Annotations: RegistrationFixture.HostAnnotations,
			Labels:      RegistrationFixture.HostLabels,
			PubKey:      string(wantPubKey),
		}

		It("should fail on pubkey marshalling error", func() {
			wantErr := errors.New("test unmarshalling pubkey error")
			id.EXPECT().MarshalPublic().Return([]byte(""), wantErr)

			err := handler.Register()
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})
		It("should register", func() {
			gomock.InOrder(
				id.EXPECT().MarshalPublic().Return(wantPubKey, nil),
				// First get registration call fails. Should repeat to recover.
				mClient.EXPECT().GetRegistration().Return(nil, errors.New("test get registration fail")),
				mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
				plugin.EXPECT().GetHostname().Return("host", nil),
				// The registration loop is trying to determine whether the ElementalHost has been created already. Error = not yet
				mClient.EXPECT().PatchHost(api.HostPatchRequest{}, HostResponseFixture.Name).Return(nil, errors.New("test not found")),
				// Let's make the first create host call fail. Expect to recover.
				mClient.EXPECT().CreateHost(wantRequest).Return(errors.New("test creat host fail")),
				mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
				// Expect a new hostname to be formatted due to creation failure.
				plugin.EXPECT().GetHostname().Return("host", nil),
				mClient.EXPECT().PatchHost(api.HostPatchRequest{}, HostResponseFixture.Name).Return(nil, errors.New("test not found")),
				mClient.EXPECT().CreateHost(wantRequest).Return(nil),
				// Expect phase to be updated
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1beta1.PhaseRegistering)}, HostResponseFixture.Name),
			)

			err := handler.Register()
			Expect(err).ToNot(HaveOccurred())
			Expect(agentContext.Hostname).To(Equal(HostResponseFixture.Name))
			Expect(agentContext.Config).To(Equal(config.FromAPI(RegistrationFixture)))
		})
		It("should not create ElementalHost twice", func() {
			wantPubKey := []byte("just a test pubkey")

			gomock.InOrder(
				id.EXPECT().MarshalPublic().Return(wantPubKey, nil),
				mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
				plugin.EXPECT().GetHostname().Return("host", nil),
				// No errors on patch request, means this host exists already and matches our current identity (due to authentication success)
				mClient.EXPECT().PatchHost(api.HostPatchRequest{}, HostResponseFixture.Name).Return(nil, nil),
				// Expect phase to be updated
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1beta1.PhaseRegistering)}, HostResponseFixture.Name),
			)

			err := handler.Register()
			Expect(err).ToNot(HaveOccurred())
			Expect(agentContext.Hostname).To(Equal(HostResponseFixture.Name))
			Expect(agentContext.Config).To(Equal(config.FromAPI(RegistrationFixture)))
		})
	})

	When("finalizing registration", func() {
		It("should install hostname and config files to finalize registration", func() {
			wantMarshalledIdentity := []byte("test identity")
			wantAgentConfigBytes, err := yaml.Marshal(agentContext.Config)
			Expect(err).ToNot(HaveOccurred())
			wantIdentityFilePath := fmt.Sprintf("%s/%s", ConfigFixture.Agent.WorkDir, identity.PrivateKeyFile)
			gomock.InOrder(
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1beta1.PhaseFinalizingRegistration)}, HostResponseFixture.Name),
				plugin.EXPECT().InstallHostname(HostResponseFixture.Name).Return(nil),
				plugin.EXPECT().InstallFile(wantAgentConfigBytes, agentContext.ConfigPath, uint32(0640), 0, 0).Return(nil),
				id.EXPECT().Marshal().Return(wantMarshalledIdentity, nil),
				plugin.EXPECT().InstallFile(wantMarshalledIdentity, wantIdentityFilePath, uint32(0640), 0, 0).Return(nil),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1beta1.RegistrationReady,
							Status:   corev1.ConditionTrue,
							Severity: clusterv1.ConditionSeverityInfo,
							Reason:   "",
							Message:  "",
						},
					))
				}),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1beta1.InstallationReady,
							Status:   corev1.ConditionFalse,
							Severity: infrastructurev1beta1.WaitingForInstallationReasonSeverity,
							Reason:   infrastructurev1beta1.WaitingForInstallationReason,
							Message:  "Host is registered successfully. Waiting for installation.",
						},
					))
				}),
			)

			Expect(handler.FinalizeRegistration()).To(Succeed())
		})
		It("should fail on finalizing registration error", func() {
			wantErr := errors.New("test finalizing registration error")

			gomock.InOrder(
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1beta1.PhaseFinalizingRegistration)}, HostResponseFixture.Name),
				plugin.EXPECT().InstallHostname(HostResponseFixture.Name).Return(wantErr),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1beta1.RegistrationReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1beta1.RegistrationFailedReason,
							Message:  "persisting hostname '" + HostResponseFixture.Name + "': " + wantErr.Error(),
						},
					))
				}),
			)

			err := handler.FinalizeRegistration()
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})
		It("should recover from update Ready condition errors", func() {
			wantMarshalledIdentity := []byte("test identity")
			wantAgentConfigBytes, err := yaml.Marshal(agentContext.Config)
			Expect(err).ToNot(HaveOccurred())
			wantIdentityFilePath := fmt.Sprintf("%s/%s", ConfigFixture.Agent.WorkDir, identity.PrivateKeyFile)
			gomock.InOrder(
				mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(infrastructurev1beta1.PhaseFinalizingRegistration)}, HostResponseFixture.Name),
				plugin.EXPECT().InstallHostname(HostResponseFixture.Name).Return(nil),
				plugin.EXPECT().InstallFile(wantAgentConfigBytes, agentContext.ConfigPath, uint32(0640), 0, 0).Return(nil),
				id.EXPECT().Marshal().Return(wantMarshalledIdentity, nil),
				plugin.EXPECT().InstallFile(wantMarshalledIdentity, wantIdentityFilePath, uint32(0640), 0, 0).Return(nil),

				// First update condition error fails, expect a second attempt
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, errors.New("test update condition error")),
				// Expect second attempt
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1beta1.RegistrationReady,
							Status:   corev1.ConditionTrue,
							Severity: clusterv1.ConditionSeverityInfo,
							Reason:   "",
							Message:  "",
						},
					))
				}),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, _ string) {
					Expect(*patch.Condition).Should(Equal(
						clusterv1.Condition{
							Type:     infrastructurev1beta1.InstallationReady,
							Status:   corev1.ConditionFalse,
							Severity: infrastructurev1beta1.WaitingForInstallationReasonSeverity,
							Reason:   infrastructurev1beta1.WaitingForInstallationReason,
							Message:  "Host is registered successfully. Waiting for installation.",
						},
					))
				}),
			)

			Expect(handler.FinalizeRegistration()).To(Succeed())
		})
	})
})
