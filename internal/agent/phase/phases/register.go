package phases

import (
	"fmt"
	"time"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RegistrationHandler interface {
	Register() (string, config.Config, error)
	FinalizeRegistration(hostname string, agentConfigPath string, agentConfig config.Config) error
}

var _ RegistrationHandler = (*registrationHandler)(nil)

func NewRegistrationHandler(client client.Client, osPlugin osplugin.Plugin, id identity.Identity, reconciliation time.Duration) RegistrationHandler {
	return &registrationHandler{
		client:         client,
		osPlugin:       osPlugin,
		id:             id,
		reconciliation: reconciliation,
	}
}

type registrationHandler struct {
	client         client.Client
	osPlugin       osplugin.Plugin
	id             identity.Identity
	reconciliation time.Duration
}

func (r *registrationHandler) Register() (string, config.Config, error) {
	pubKey, err := r.id.MarshalPublic()
	if err != nil {
		return "", config.Config{}, fmt.Errorf("marshalling host public key: %w", err)
	}
	hostname, config := r.registrationLoop(pubKey)
	log.Infof("Successfully registered as '%s'", hostname)
	return hostname, config, nil
}

func (r *registrationHandler) FinalizeRegistration(hostname string, configPath string, agentConfig config.Config) error {
	err := r.finalize(hostname, configPath, agentConfig)
	if err != nil {
		updateCondition(r.client, hostname, clusterv1.Condition{
			Type:     infrastructurev1beta1.RegistrationReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.RegistrationFailedReason,
			Message:  err.Error(),
		})
		return fmt.Errorf("finalizing registration: %w", err)
	}
	err = updateConditionOrFail(r.client, hostname, clusterv1.Condition{
		Type:     infrastructurev1beta1.RegistrationReady,
		Status:   corev1.ConditionTrue,
		Severity: clusterv1.ConditionSeverityInfo,
	})
	if err != nil {
		return fmt.Errorf("updating RegistrationReady True condition: %w", err)
	}
	updateCondition(r.client, hostname, clusterv1.Condition{
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
	if err := r.osPlugin.InstallHostname(hostname); err != nil {
		return fmt.Errorf("persisting hostname '%s': %w", hostname, err)
	}
	// Persist agent config
	agentConfigBytes, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("marshalling agent config: %w", err)
	}
	if err := r.osPlugin.InstallFile(agentConfigBytes, configPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting agent config file '%s': %w", configPath, err)
	}
	// Persist identity file
	identityBytes, err := r.id.Marshal()
	if err != nil {
		return fmt.Errorf("marshalling identity: %w", err)
	}
	privateKeyPath := fmt.Sprintf("%s/%s", agentConfig.Agent.WorkDir, identity.PrivateKeyFile)
	if err := r.osPlugin.InstallFile(identityBytes, privateKeyPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting private key file '%s': %w", privateKeyPath, err)
	}
	return nil
}

// registrationLoop **indefinitely** tries to fetch the remote registration and register a new ElementalHost.
func (r *registrationHandler) registrationLoop(pubKey []byte) (string, config.Config) {
	hostnameFormatter := hostname.NewFormatter(r.osPlugin)
	var newHostname string
	var registration *api.RegistrationResponse
	var err error
	registrationError := false
	for {
		// Wait for recovery
		if registrationError {
			log.Debugf("Waiting '%s' on registration error to recover", r.reconciliation)
			time.Sleep(r.reconciliation)
		}
		// Fetch remote Registration
		log.Debug("Fetching remote registration")
		registration, err = r.client.GetRegistration()
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
		if _, err := r.client.PatchHost(api.HostPatchRequest{}, newHostname); err == nil {
			log.Infof("Found existing ElementalHost: %s. Skipping new registration.", newHostname)
			break
		}
		// Register new Elemental Host
		log.Debugf("Registering new host: %s", newHostname)
		if err := r.client.CreateHost(api.HostCreateRequest{
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
