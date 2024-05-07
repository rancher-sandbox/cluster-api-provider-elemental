package phase

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
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RegistrationHandler interface {
	Register() (string, error)
	FinalizeRegistration(hostname string, configPath string) error
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

func (r *registrationHandler) Register() (string, error) {
	pubKey, err := r.id.MarshalPublic()
	if err != nil {
		return "", fmt.Errorf("marshalling host public key: %w", err)
	}
	hostname := r.registrationLoop(pubKey)
	log.Infof("Successfully registered as '%s'", hostname)
	return hostname, nil
}

func (r *registrationHandler) FinalizeRegistration(hostname string, configPath string) error {
	err := r.finalize(hostname, configPath)
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
	updateCondition(r.client, hostname, clusterv1.Condition{
		Type:     infrastructurev1beta1.RegistrationReady,
		Status:   corev1.ConditionTrue,
		Severity: clusterv1.ConditionSeverityInfo,
	})
	updateCondition(r.client, hostname, clusterv1.Condition{
		Type:     infrastructurev1beta1.InstallationReady,
		Status:   corev1.ConditionFalse,
		Severity: infrastructurev1beta1.WaitingForInstallationReasonSeverity,
		Reason:   infrastructurev1beta1.WaitingForInstallationReason,
		Message:  "Host is registered successfully. Waiting for installation.",
	})
	return nil
}

func (r *registrationHandler) finalize(hostname string, configPath string) error {
	// Persist registered hostname
	if err := r.osPlugin.InstallHostname(hostname); err != nil {
		return fmt.Errorf("persisting hostname '%s': %w", hostname, err)
	}
	// Fetch remote Registration
	log.Debug("Fetching remote registration")
	registration, err := r.client.GetRegistration()
	if err != nil {
		return fmt.Errorf("getting remote Registration: %w", err)
	}
	// Persist agent config
	agentConfig := config.FromAPI(*registration)
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

// registrationLoop **indefinitely** tries to fetch the remote registration and register a new ElementalHost
func (r *registrationHandler) registrationLoop(pubKey []byte) string {
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
	return newHostname
}
