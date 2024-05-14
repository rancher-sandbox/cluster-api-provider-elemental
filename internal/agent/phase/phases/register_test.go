package phases

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	gomock "go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("registration handler", Label("cli", "phases", "registration"), func() {
	var mockCtrl *gomock.Controller
	var mClient *client.MockClient
	var plugin *osplugin.MockPlugin
	var id *identity.MockIdentity
	var handler RegistrationHandler

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mClient = client.NewMockClient(mockCtrl)
		plugin = osplugin.NewMockPlugin(mockCtrl)
		id = identity.NewMockIdentity(mockCtrl)
		handler = NewRegistrationHandler(mClient, plugin, id, time.Microsecond)
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

			_, err := handler.Register()
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
			)

			hostname, err := handler.Register()
			Expect(err).ToNot(HaveOccurred())
			Expect(hostname).To(Equal(HostResponseFixture.Name))
		})
		It("should not create ElementalHost twice", func() {
			wantPubKey := []byte("just a test pubkey")

			gomock.InOrder(
				id.EXPECT().MarshalPublic().Return(wantPubKey, nil),
				mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
				plugin.EXPECT().GetHostname().Return("host", nil),
				// No errors on patch request, means this host exists already and matches our current identity (due to authentication success)
				mClient.EXPECT().PatchHost(api.HostPatchRequest{}, HostResponseFixture.Name).Return(nil, nil),
				// Nothing to do here anymore, registration loop should break.
			)

			hostname, err := handler.Register()
			Expect(err).ToNot(HaveOccurred())
			Expect(hostname).To(Equal(HostResponseFixture.Name))
		})
	})

	When("finalizing registration", func() {
		wantConfigPath := "/test/config/path"
		wantMarshalledIdentity := []byte("test identity")
		wantAgentConfig := config.FromAPI(RegistrationFixture)
		wantAgentConfigBytes, err := yaml.Marshal(wantAgentConfig)
		Expect(err).ToNot(HaveOccurred())
		wantIdentityFilePath := fmt.Sprintf("%s/%s", RegistrationFixture.Config.Elemental.Agent.WorkDir, identity.PrivateKeyFile)

		It("should install hostname and config files to finalize registration", func() {
			gomock.InOrder(
				plugin.EXPECT().InstallHostname(HostResponseFixture.Name).Return(nil),
				mClient.EXPECT().GetRegistration().Return(&RegistrationFixture, nil),
				plugin.EXPECT().InstallFile(wantAgentConfigBytes, wantConfigPath, uint32(0640), 0, 0).Return(nil),
				id.EXPECT().Marshal().Return(wantMarshalledIdentity, nil),
				plugin.EXPECT().InstallFile(wantMarshalledIdentity, wantIdentityFilePath, uint32(0640), 0, 0).Return(nil),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
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
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
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

			err := handler.FinalizeRegistration(HostResponseFixture.Name, wantConfigPath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail on finalizing registration error", func() {
			wantErr := errors.New("test finalizing registration error")

			gomock.InOrder(
				plugin.EXPECT().InstallHostname(HostResponseFixture.Name).Return(wantErr),
				mClient.EXPECT().PatchHost(gomock.Any(), HostResponseFixture.Name).Return(nil, nil).Do(func(patch api.HostPatchRequest, hostName string) {
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

			err := handler.FinalizeRegistration(HostResponseFixture.Name, wantConfigPath)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})

	})
})
