package phase

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase/phases"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	gomock "go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Host Phases Handler Suite")
}

var _ = Describe("handler", Label("cli", "phases", "handler"), func() {
	var mockCtrl *gomock.Controller
	var phaseHandler hostPhaseHandler
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})
	When("operating normally", func() {
		It("should not return error when updating remote status fails", func() {
			mClient := client.NewMockClient(mockCtrl)
			hostPhaseHandler := hostPhaseHandler{client: mClient}
			hostPhaseHandler.hostContext.Hostname = "just a test hostname"

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseRunning)}, hostPhaseHandler.hostContext.Hostname).Return(nil, errors.New("test patch error"))

			_, err := hostPhaseHandler.Handle(v1beta1.PhaseRunning)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should return error on unknown phase", func() {
			hostPhaseHandler := NewHostPhaseHandler()
			_, err := hostPhaseHandler.Handle(v1beta1.HostPhase("test uknown"))
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ErrUknownPhase)).To(BeTrue())
		})
	})

	When("registering", func() {
		var registrationHandler *phases.MockRegistrationHandler
		var mClient *client.MockClient
		BeforeEach(func() {
			mClient = client.NewMockClient(mockCtrl)
			registrationHandler = phases.NewMockRegistrationHandler(mockCtrl)
			phaseHandler = hostPhaseHandler{
				client:   mClient,
				register: registrationHandler,
			}
		})
		It("should set hostname after registration", func() {
			wantHostname := "just a test hostname"

			registrationHandler.EXPECT().Register().Return(wantHostname, nil)
			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseRegistering)}, wantHostname).Return(nil, nil)

			post, err := phaseHandler.Handle(v1beta1.PhaseRegistering)
			Expect(err).ToNot(HaveOccurred())
			Expect(post).To(Equal(phases.PostAction{}))

			Expect(phaseHandler.hostContext.Hostname).To(Equal(wantHostname), "Context Hostname should be updated with registered Hostname value")
		})
		It("should fail on registration error", func() {
			wantErr := errors.New("test registration error")
			registrationHandler.EXPECT().Register().Return("", wantErr)

			_, err := phaseHandler.Handle(v1beta1.PhaseRegistering)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})
		It("should pass agentConfig and hostname when finalizing registration", func() {
			phaseHandler.hostContext.Hostname = "just a test hostname"
			phaseHandler.hostContext.AgentConfigPath = "/just/a/test/path"

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseFinalizingRegistration)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			registrationHandler.EXPECT().FinalizeRegistration(phaseHandler.hostContext.Hostname, phaseHandler.hostContext.AgentConfigPath).Return(nil)

			post, err := phaseHandler.Handle(v1beta1.PhaseFinalizingRegistration)
			Expect(err).ToNot(HaveOccurred())
			Expect(post).To(Equal(phases.PostAction{}))
		})
		It("should fail on finalizing registration error", func() {
			phaseHandler.hostContext.Hostname = "just a test hostname"
			phaseHandler.hostContext.AgentConfigPath = "/just/a/test/path"
			wantErr := errors.New("test finalizing registration error")

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseFinalizingRegistration)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			registrationHandler.EXPECT().FinalizeRegistration(phaseHandler.hostContext.Hostname, phaseHandler.hostContext.AgentConfigPath).Return(wantErr)

			_, err := phaseHandler.Handle(v1beta1.PhaseFinalizingRegistration)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})
	})

	When("installing", func() {
		var installationHandler *phases.MockInstallHandler
		var mClient *client.MockClient
		BeforeEach(func() {
			mClient = client.NewMockClient(mockCtrl)
			installationHandler = phases.NewMockInstallHandler(mockCtrl)
			phaseHandler = hostPhaseHandler{
				client:  mClient,
				install: installationHandler,
			}
		})
		It("should return post conditions based on agent config", func() {
			phaseHandler.hostContext.Hostname = "just a test hostname"
			phaseHandler.hostContext.AgentConfig.Agent.PostInstall.PowerOff = true
			phaseHandler.hostContext.AgentConfig.Agent.PostInstall.Reboot = true

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseInstalling)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			installationHandler.EXPECT().Install(phaseHandler.hostContext.Hostname)

			post, err := phaseHandler.Handle(v1beta1.PhaseInstalling)
			Expect(err).ToNot(HaveOccurred())
			Expect(post).To(Equal(phases.PostAction{PowerOff: true, Reboot: true}))
		})
	})

	When("bootstrapping", func() {
		var bootstrapHandler *phases.MockBootstrapHandler
		var mClient *client.MockClient
		BeforeEach(func() {
			mClient = client.NewMockClient(mockCtrl)
			bootstrapHandler = phases.NewMockBootstrapHandler(mockCtrl)
			phaseHandler = hostPhaseHandler{
				client:    mClient,
				bootstrap: bootstrapHandler,
			}
		})
		It("should return post conditions based on bootstrap results", func() {
			wantPost := phases.PostAction{PowerOff: true, Reboot: true}

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseBootstrapping)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			bootstrapHandler.EXPECT().Bootstrap(phaseHandler.hostContext.Hostname).Return(wantPost, nil)

			post, err := phaseHandler.Handle(v1beta1.PhaseBootstrapping)
			Expect(err).ToNot(HaveOccurred())
			Expect(post).To(Equal(wantPost))
		})
		It("should fail on bootstrap error", func() {
			wantErr := errors.New("test bootstrap error")
			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseBootstrapping)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			bootstrapHandler.EXPECT().Bootstrap(phaseHandler.hostContext.Hostname).Return(phases.PostAction{}, wantErr)

			_, err := phaseHandler.Handle(v1beta1.PhaseBootstrapping)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})
	})

	When("resetting", func() {
		var resetHandler *phases.MockResetHandler
		var mClient *client.MockClient
		BeforeEach(func() {
			mClient = client.NewMockClient(mockCtrl)
			resetHandler = phases.NewMockResetHandler(mockCtrl)
			phaseHandler = hostPhaseHandler{
				client: mClient,
				reset:  resetHandler,
			}
		})
		It("should return post conditions based on agent config", func() {
			phaseHandler.hostContext.Hostname = "just a test hostname"
			phaseHandler.hostContext.AgentConfig.Agent.PostReset.PowerOff = true
			phaseHandler.hostContext.AgentConfig.Agent.PostReset.Reboot = true

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseResetting)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			resetHandler.EXPECT().Reset(phaseHandler.hostContext.Hostname)

			post, err := phaseHandler.Handle(v1beta1.PhaseResetting)
			Expect(err).ToNot(HaveOccurred())
			Expect(post).To(Equal(phases.PostAction{PowerOff: true, Reboot: true}))
		})
		It("should trigger reset", func() {
			phaseHandler.hostContext.Hostname = "just a test hostname"

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseTriggeringReset)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			resetHandler.EXPECT().TriggerReset(phaseHandler.hostContext.Hostname).Return(nil)

			post, err := phaseHandler.Handle(v1beta1.PhaseTriggeringReset)
			Expect(err).ToNot(HaveOccurred())
			Expect(post).To(Equal(phases.PostAction{}))
		})
		It("should fail on trigger reset error", func() {
			phaseHandler.hostContext.Hostname = "just a test hostname"
			wantErr := errors.New("test trigger reset error")

			mClient.EXPECT().PatchHost(api.HostPatchRequest{Phase: ptr.To(v1beta1.PhaseTriggeringReset)}, phaseHandler.hostContext.Hostname).Return(nil, nil)
			resetHandler.EXPECT().TriggerReset(phaseHandler.hostContext.Hostname).Return(wantErr)

			_, err := phaseHandler.Handle(v1beta1.PhaseTriggeringReset)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, wantErr)).To(BeTrue())
		})
	})
})
