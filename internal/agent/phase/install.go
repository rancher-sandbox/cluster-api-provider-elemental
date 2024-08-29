package phase

import (
	corev1 "k8s.io/api/core/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"encoding/json"
	"fmt"
	"time"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"k8s.io/utils/ptr"
)

type InstallHandler interface {
	Install()
}

var _ InstallHandler = (*installHandler)(nil)

func NewInstallHandler(agentContext context.AgentContext) InstallHandler {
	return &installHandler{
		agentContext: agentContext,
	}
}

type installHandler struct {
	agentContext context.AgentContext
}

func (i *installHandler) Install() {
	setPhase(i.agentContext.Client, i.agentContext.Hostname, infrastructurev1beta1.PhaseInstalling)
	i.installLoop()
}

// installLoop **indefinitely** tries to fetch the remote registration and install the ElementalHost.
func (i *installHandler) installLoop() {
	cloudConfigAlreadyApplied := false
	alreadyInstalled := false
	var installationError error
	installationErrorReason := infrastructurev1beta1.InstallationFailedReason
	for {
		if installationError != nil {
			// Log error
			log.Error(installationError, "installing host")
			// Attempt to report failed condition on management server
			updateCondition(i.agentContext.Client, i.agentContext.Hostname, clusterv1.Condition{
				Type:     infrastructurev1beta1.InstallationReady,
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   installationErrorReason,
				Message:  installationError.Error(),
			})
			// Clear error for next attempt
			installationError = nil
			installationErrorReason = infrastructurev1beta1.InstallationFailedReason
			// Wait for recovery (end user may fix the remote installation instructions meanwhile)
			log.Debugf("Waiting '%s' on installation error for installation instructions to mutate", i.agentContext.Config.Agent.Reconciliation)
			time.Sleep(i.agentContext.Config.Agent.Reconciliation)
		}
		// Fetch remote Registration
		var registration *api.RegistrationResponse
		var err error
		if !cloudConfigAlreadyApplied || !alreadyInstalled {
			log.Debug("Fetching remote registration")
			registration, err = i.agentContext.Client.GetRegistration()
			if err != nil {
				installationError = fmt.Errorf("getting remote Registration: %w", err)
				continue
			}
		}
		// Apply Cloud Config
		if !cloudConfigAlreadyApplied {
			cloudConfigBytes, err := json.Marshal(registration.Config.CloudConfig)
			if err != nil {
				installationError = fmt.Errorf("marshalling cloud config: %w", err)
				installationErrorReason = infrastructurev1beta1.CloudConfigInstallationFailedReason
				continue
			}
			if err := i.agentContext.Plugin.InstallCloudInit(cloudConfigBytes); err != nil {
				installationError = fmt.Errorf("installing cloud config: %w", err)
				installationErrorReason = infrastructurev1beta1.CloudConfigInstallationFailedReason
				continue
			}
			cloudConfigAlreadyApplied = true
		}
		// Install
		if !alreadyInstalled {
			installBytes, err := json.Marshal(registration.Config.Elemental.Install)
			if err != nil {
				installationError = fmt.Errorf("marshalling install config: %w", err)
				continue
			}
			if err := i.agentContext.Plugin.Install(installBytes); err != nil {
				installationError = fmt.Errorf("installing host: %w", err)
				continue
			}
			alreadyInstalled = true
		}
		// Report installation success
		patchRequest := api.HostPatchRequest{Installed: ptr.To(true)}
		patchRequest.SetCondition(infrastructurev1beta1.InstallationReady,
			corev1.ConditionTrue,
			clusterv1.ConditionSeverityInfo,
			"", "")
		if _, err := i.agentContext.Client.PatchHost(patchRequest, i.agentContext.Hostname); err != nil {
			installationError = fmt.Errorf("patching host with installation successful: %w", err)
			continue
		}
		break
	}
}
