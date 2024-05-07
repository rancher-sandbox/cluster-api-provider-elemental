package phase

import (
	corev1 "k8s.io/api/core/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"encoding/json"
	"fmt"
	"time"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"k8s.io/utils/ptr"
)

type InstallHandler interface {
	Install(hostname string)
}

var _ InstallHandler = (*installHandler)(nil)

func NewInstallHandler(client client.Client, osPlugin osplugin.Plugin, id identity.Identity, reconciliation time.Duration) InstallHandler {
	return &installHandler{
		client:         client,
		osPlugin:       osPlugin,
		id:             id,
		reconciliation: reconciliation,
	}
}

type installHandler struct {
	client         client.Client
	osPlugin       osplugin.Plugin
	id             identity.Identity
	reconciliation time.Duration
}

func (i *installHandler) Install(hostname string) {
	i.installLoop(hostname)
}

// installLoop **indefinitely** tries to fetch the remote registration and install the ElementalHost
func (i *installHandler) installLoop(hostname string) {
	cloudConfigAlreadyApplied := false
	alreadyInstalled := false
	var installationError error
	installationErrorReason := infrastructurev1beta1.InstallationFailedReason
	for {
		if installationError != nil {
			// Log error
			log.Error(installationError, "installing host")
			// Attempt to report failed condition on management server
			updateCondition(i.client, hostname, clusterv1.Condition{
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
			log.Debugf("Waiting '%s' on installation error for installation instructions to mutate", i.reconciliation)
			time.Sleep(i.reconciliation)
		}
		// Fetch remote Registration
		var registration *api.RegistrationResponse
		var err error
		if !cloudConfigAlreadyApplied || !alreadyInstalled {
			log.Debug("Fetching remote registration")
			registration, err = i.client.GetRegistration()
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
			if err := i.osPlugin.InstallCloudInit(cloudConfigBytes); err != nil {
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
			if err := i.osPlugin.Install(installBytes); err != nil {
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
		if _, err := i.client.PatchHost(patchRequest, hostname); err != nil {
			installationError = fmt.Errorf("patching host with installation successful: %w", err)
			continue
		}
		break
	}
}
