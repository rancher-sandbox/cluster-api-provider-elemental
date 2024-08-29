package phase

import (
	"fmt"
	"time"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RegistrationHandler interface {
	Register() error
	FinalizeRegistration() error
}

var _ RegistrationHandler = (*registrationHandler)(nil)

func NewRegistrationHandler(agentContext *context.AgentContext) RegistrationHandler {
	return &registrationHandler{
		agentContext: agentContext,
	}
}

type registrationHandler struct {
	agentContext *context.AgentContext
}

func (r *registrationHandler) Register() error {
	pubKey, err := r.agentContext.Identity.MarshalPublic()
	if err != nil {
		return fmt.Errorf("marshalling host public key: %w", err)
	}
	hostname, config := r.registrationLoop(pubKey)
	log.Infof("Successfully registered as '%s'", hostname)
	r.agentContext.Hostname = hostname
	r.agentContext.Config = config
	setPhase(r.agentContext.Client, r.agentContext.Hostname, infrastructurev1beta1.PhaseRegistering) // Note that we set the phase **after* its conclusion, because we do not have any remote ElementalHost to patch before.
	return nil
}

func (r *registrationHandler) FinalizeRegistration() error {
	setPhase(r.agentContext.Client, r.agentContext.Hostname, infrastructurev1beta1.PhaseFinalizingRegistration)
	err := r.finalize(r.agentContext.Hostname, r.agentContext.ConfigPath, r.agentContext.Config)
	if err != nil {
		updateCondition(r.agentContext.Client, r.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1beta1.RegistrationReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.RegistrationFailedReason,
			Message:  err.Error(),
		})
		return fmt.Errorf("finalizing registration: %w", err)
	}

	// We try to catch and recover errors here since this is not recoverable once the cli exits with an error.
	//
	// If this steps fail and `elemental-agent --register` is called again, it will try to register using a new identity,
	// since the system is not installed yet and the previously registered identity lived in-memory.
	//
	// Therefore we must prevent the entire registration process from failing on recoverable errors (in this case a network issue).
	for {
		if err := updateConditionOrFail(r.agentContext.Client, r.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1beta1.RegistrationReady,
			Status:   corev1.ConditionTrue,
			Severity: clusterv1.ConditionSeverityInfo,
		}); err != nil {
			log.Error(err, "updating RegistrationReady True condition")
			log.Debugf("Waiting '%s' on update condition error to recover", r.agentContext.Config.Agent.Reconciliation)
			time.Sleep(r.agentContext.Config.Agent.Reconciliation)
			continue
		}
		break
	}

	updateCondition(r.agentContext.Client, r.agentContext.Hostname, clusterv1.Condition{
		Type:     infrastructurev1beta1.InstallationReady,
		Status:   corev1.ConditionFalse,
		Severity: infrastructurev1beta1.WaitingForInstallationReasonSeverity,
		Reason:   infrastructurev1beta1.WaitingForInstallationReason,
		Message:  "Host is registered successfully. Waiting for installation.",
	})
	return nil
}

func (r *registrationHandler) finalize(hostname string, configPath string, agentConfig config.Config) error {
	// Persist registered hostname
	if err := r.agentContext.Plugin.InstallHostname(hostname); err != nil {
		return fmt.Errorf("persisting hostname '%s': %w", hostname, err)
	}
	// Persist agent config
	agentConfigBytes, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("marshalling agent config: %w", err)
	}
	if err := r.agentContext.Plugin.InstallFile(agentConfigBytes, configPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting agent config file '%s': %w", configPath, err)
	}
	// Persist identity file
	identityBytes, err := r.agentContext.Identity.Marshal()
	if err != nil {
		return fmt.Errorf("marshalling identity: %w", err)
	}
	privateKeyPath := fmt.Sprintf("%s/%s", agentConfig.Agent.WorkDir, identity.PrivateKeyFile)
	if err := r.agentContext.Plugin.InstallFile(identityBytes, privateKeyPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting private key file '%s': %w", privateKeyPath, err)
	}
	return nil
}

// registrationLoop **indefinitely** tries to fetch the remote registration and register a new ElementalHost.
func (r *registrationHandler) registrationLoop(pubKey []byte) (string, config.Config) {
	hostnameFormatter := hostname.NewFormatter(r.agentContext.Plugin)
	var newHostname string
	var registration *api.RegistrationResponse
	var err error
	registrationError := false
	for {
		// Wait for recovery
		if registrationError {
			log.Debugf("Waiting '%s' on registration error to recover", r.agentContext.Config.Agent.Reconciliation)
			time.Sleep(r.agentContext.Config.Agent.Reconciliation)
		}
		// Fetch remote Registration
		log.Debug("Fetching remote registration")
		registration, err = r.agentContext.Client.GetRegistration()
		if err != nil {
			log.Error(err, "getting remote Registration")
			registrationError = true
			continue
		}
		// Pick a new hostname
		// There is a tiny chance the random hostname generation will collide with existing ones.
		// It's safer to generate a new one in case of host creation failure.
		newHostname, err = hostnameFormatter.FormatHostname(registration.Config.Elemental.Agent.Hostname)
		log.Debugf("Selected hostname: %s", newHostname)
		if err != nil {
			log.Error(err, "picking new hostname")
			registrationError = true
			continue
		}
		// Check if Registration already happened
		// This can happen if finalizing the registration failed and the agent is started again to re-attempt.
		if _, err := r.agentContext.Client.PatchHost(api.HostPatchRequest{}, newHostname); err == nil {
			log.Infof("Found existing ElementalHost: %s. Skipping new registration.", newHostname)
			break
		}
		// Register new Elemental Host
		log.Debugf("Registering new host: %s", newHostname)
		if err := r.agentContext.Client.CreateHost(api.HostCreateRequest{
			Name:        newHostname,
			Annotations: registration.HostAnnotations,
			Labels:      registration.HostLabels,
			PubKey:      string(pubKey),
		}); err != nil {
			log.Error(err, "registering new ElementalHost")
			registrationError = true
			continue
		}
		break
	}

	return newHostname, config.FromAPI(*registration)
}
