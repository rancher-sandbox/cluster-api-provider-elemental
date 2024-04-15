package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfsafero/v4"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname"
	log "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
)

const (
	configPathDefault     = "/etc/elemental/agent/config.yaml"
	bootstrapSentinelFile = "/run/cluster-api/bootstrap-success.complete"
)

// Flags.
var (
	versionFlag  bool
	resetFlag    bool
	installFlag  bool
	registerFlag bool
	debugFlag    bool
)

// Arguments.
var (
	configPath string
)

var (
	ErrIncorrectArguments = errors.New("incorrect arguments, run 'elemental-agent --help' for usage")
)

func main() {
	fs := vfs.OSFS
	osPluginLoader := osplugin.NewLoader()
	client := client.NewClient(version.Version)
	cmd := newCommand(fs, osPluginLoader, client)
	if err := cmd.Execute(); err != nil {
		log.Error(err, "running elemental-agent")
		os.Exit(1)
	}
}

func newCommand(fs vfs.FS, pluginLoader osplugin.Loader, client client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "elemental-agent",
		Short: "Elemental Agent command",
		Long:  "elemental-agent registers a node with the elemental-operator via a config file",
		RunE: func(_ *cobra.Command, args []string) error {
			// Display version
			if versionFlag {
				log.Infof("Agent version %s, commit %s, commit date %s", version.Version, version.Commit, version.CommitDate)
				return nil
			}
			// Sanity checks
			if installFlag && resetFlag {
				return fmt.Errorf("--install and --reset are mutually exclusive: %w", ErrIncorrectArguments)
			}
			// Parse config file
			conf, err := getConfig(fs)
			if err != nil {
				return fmt.Errorf("parsing configuration file '%s': %w", configPath, err)
			}
			// Set debug logs
			if conf.Agent.Debug || debugFlag {
				log.EnableDebug()
				log.Debug("Debug logging enabled")
			}
			// Initialize WorkDir
			if err := utils.CreateDirectory(fs, conf.Agent.WorkDir); err != nil {
				return fmt.Errorf("creating work directory '%s': %w", conf.Agent.WorkDir, err)
			}
			// Initialize Plugin
			log.Infof("Loading Plugin: %s", conf.Agent.OSPlugin)
			osPlugin, err := pluginLoader.Load(conf.Agent.OSPlugin)
			if err != nil {
				return fmt.Errorf("Loading plugin '%s': %w", conf.Agent.OSPlugin, err)
			}
			log.Info("Initializing Plugin")
			if err := osPlugin.Init(osplugin.PluginContext{
				WorkDir:    conf.Agent.WorkDir,
				ConfigPath: configPath,
				Debug:      conf.Agent.Debug || debugFlag,
			}); err != nil {
				return fmt.Errorf("Initializing plugin: %w", err)
			}
			// Initialize Identity
			identityManager := identity.NewManager(fs, conf.Agent.WorkDir)
			identity, err := identityManager.LoadSigningKeyOrCreateNew()
			if err != nil {
				return fmt.Errorf("initializing identity: %w", err)
			}
			// Initialize Elemental API Client
			if err := client.Init(fs, identity, conf); err != nil {
				return fmt.Errorf("initializing Elemental API client: %w", err)
			}
			// Get current hostname
			hostname, err := osPlugin.GetHostname()
			if err != nil {
				return fmt.Errorf("getting current hostname: %w", err)
			}
			// Register
			if registerFlag {
				log.Info("Registering Elemental Host")
				pubKey, err := identity.MarshalPublic()
				if err != nil {
					return fmt.Errorf("marshalling host public key: %w", err)
				}
				var registration *api.RegistrationResponse
				hostname, registration = handleRegistration(client, osPlugin, pubKey, conf.Registration.Token, conf.Agent.Reconciliation)
				log.Infof("Successfully registered as '%s'", hostname)
				if err := handlePostRegistration(osPlugin, hostname, identity, registration); err != nil {
					err = fmt.Errorf("handling post registration: %w", err)
					attemptConditionReporting(client, hostname, clusterv1.Condition{
						Type:     infrastructurev1beta1.RegistrationReady,
						Status:   corev1.ConditionFalse,
						Severity: clusterv1.ConditionSeverityError,
						Reason:   infrastructurev1beta1.RegistrationFailedReason,
						Message:  err.Error(),
					})
					return err
				}
				attemptConditionReporting(client, hostname, clusterv1.Condition{
					Type:     infrastructurev1beta1.RegistrationReady,
					Status:   corev1.ConditionTrue,
					Severity: clusterv1.ConditionSeverityInfo,
					Reason:   "",
					Message:  "",
				})
				attemptConditionReporting(client, hostname, clusterv1.Condition{
					Type:     infrastructurev1beta1.InstallationReady,
					Status:   corev1.ConditionFalse,
					Severity: infrastructurev1beta1.WaitingForInstallationReasonSeverity,
					Reason:   infrastructurev1beta1.WaitingForInstallationReason,
					Message:  "Host is registered successfully. Waiting for installation.",
				})
				// Exit program if --install was not called
				if !installFlag {
					return nil
				}
			}
			// Install
			if installFlag {
				log.Info("Installing Elemental")
				handleInstall(client, osPlugin, hostname, conf.Registration.Token, conf.Agent.Reconciliation)
				log.Info("Installation successful")
				handlePost(osPlugin, conf.Agent.PostInstall.PowerOff, conf.Agent.PostInstall.Reboot)
				return nil
			}

			// Reset
			if resetFlag {
				log.Info("Resetting Elemental")
				handleReset(client, osPlugin, hostname, conf.Registration.Token, conf.Agent.Reconciliation)
				log.Info("Reset successful")
				handlePost(osPlugin, conf.Agent.PostReset.PowerOff, conf.Agent.PostReset.Reboot)
				return nil
			}

			// Normal reconcile
			log.Info("Entering reconciliation loop")
			for {
				// Patch the host and receive the patched remote host back
				log.Debug("Patching host")
				host, err := client.PatchHost(api.HostPatchRequest{}, hostname)
				if err != nil {
					log.Error(err, "patching ElementalHost during normal reconcile")
					log.Debugf("Waiting %s...", conf.Agent.Reconciliation.String())
					time.Sleep(conf.Agent.Reconciliation)
					continue
				}

				// Handle Reset trigger
				//
				// Reset should always be prioritized in the normal reconcile loop,
				// to allow reset of machines that are otherwise stuck in other phases,
				// like bootstrapping.
				if host.NeedsReset {
					log.Info("Triggering reset")
					if err := osPlugin.TriggerReset(); err != nil {
						log.Error(err, "triggering reset")
						err := fmt.Errorf("triggering reset: %w", err)
						attemptConditionReporting(client, hostname, clusterv1.Condition{
							Type:     infrastructurev1beta1.ResetReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1beta1.ResetFailedReason,
							Message:  err.Error(),
						})
						continue
					}
					attemptConditionReporting(client, hostname, clusterv1.Condition{
						Type:     infrastructurev1beta1.ResetReady,
						Status:   corev1.ConditionFalse,
						Severity: infrastructurev1beta1.WaitingForResetReasonSeverity,
						Reason:   infrastructurev1beta1.WaitingForResetReason,
						Message:  "Reset was triggered successfully. Waiting for host to reset.",
					})
					// If Reset was triggered successfully, exit the program.
					log.Info("Reset was triggered successfully. Exiting program.")
					return nil
				}

				// Handle Upgrade (PoC version)
				if !host.Bootstrapped || host.InPlaceUpgrade == infrastructurev1beta1.InPlaceUpgradePending {
					// Set OSVersionReady false condition to highlight the process started
					patchRequest := api.HostPatchRequest{}
					patchRequest.SetCondition(infrastructurev1beta1.OSVersionReady,
						corev1.ConditionFalse,
						infrastructurev1beta1.WaitingForOSVersionReconcileReasonSeverity,
						infrastructurev1beta1.WaitingForOSVersionReconcileReason,
						"Reconciling OS Version.")
					if _, err := client.PatchHost(patchRequest, hostname); err != nil {
						log.Error(err, "patching host with false OSVersionReady condition")
						time.Sleep(conf.Agent.Reconciliation)
						continue
					}

					// Serialize input to JSON
					bytes, err := json.Marshal(host.OSVersionManagement)
					if err != nil {
						log.Error(err, "marshalling Host OSVersionManagement to JSON")
						err := fmt.Errorf("marshalling Host OSVersionManagement to JSON: %w", err)
						attemptConditionReporting(client, hostname, clusterv1.Condition{
							Type:     infrastructurev1beta1.OSVersionReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1beta1.OSVersionReconciliationFailedReason,
							Message:  err.Error(),
						})
						time.Sleep(conf.Agent.Reconciliation)
						continue
					}

					// Ask the OSPlugin to reconcile
					reboot, err := osPlugin.ReconcileOSVersion(bytes)
					if err != nil {
						log.Error(err, "reconciling Host OS Version")
						err := fmt.Errorf("reconciling Host OS Version: %w", err)
						attemptConditionReporting(client, hostname, clusterv1.Condition{
							Type:     infrastructurev1beta1.OSVersionReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1beta1.OSVersionReconciliationFailedReason,
							Message:  err.Error(),
						})
						time.Sleep(conf.Agent.Reconciliation)
						continue
					}

					if reboot {
						log.Info("Rebooting after OS Version reconciliation.")
						attemptConditionReporting(client, hostname, clusterv1.Condition{
							Type:     infrastructurev1beta1.OSVersionReady,
							Status:   corev1.ConditionFalse,
							Severity: infrastructurev1beta1.WaitingForPostReconcileRebootReasonSeverity,
							Reason:   infrastructurev1beta1.WaitingForPostReconcileRebootReason,
							Message:  "Waiting for Host to reboot after OS Version has been reconciled.",
						})
						if err := osPlugin.Reboot(); err != nil {
							// Exit the program in case of reboot failures
							// Assume this is not recoverable and requires manual intervention
							return fmt.Errorf("rebooting system after OS Version reconciliation: %w", err)
						}
						return nil
					}

					// If we are here it means:
					// 1. The OSVersion input was applied successfully by the plugin.
					// 2. The Host does not need to reboot.
					//
					// We are almost ready to proceed with bootstrapping (or to continue operation in case of in-place upgrades)
					// Last thing we need is to mark the OSVersionReady to tell the provider the process has finished.
					// If this request fails we must re-try it until it succeeds before we bootstrap.
					patchRequest = api.HostPatchRequest{}
					if host.InPlaceUpgrade == infrastructurev1beta1.InPlaceUpgradePending {
						upgradeDone := infrastructurev1beta1.InPlaceUpgradeDone
						patchRequest.InPlaceUpgrade = &upgradeDone
					}
					patchRequest.SetCondition(infrastructurev1beta1.OSVersionReady,
						corev1.ConditionTrue,
						clusterv1.ConditionSeverityInfo,
						"", "")
					if _, err := client.PatchHost(patchRequest, hostname); err != nil {
						log.Error(err, "patching host with successful OSVersionReady condition")
						time.Sleep(conf.Agent.Reconciliation)
						continue
					}
				}

				// Handle bootstrap if needed
				if host.BootstrapReady && !host.Bootstrapped {
					log.Debug("Handling bootstrap application")
					exit, err := handleBootstrap(fs, client, osPlugin, hostname)
					if err != nil {
						log.Error(err, "handling bootstrap")
						attemptConditionReporting(client, hostname, clusterv1.Condition{
							Type:     infrastructurev1beta1.BootstrapReady,
							Status:   corev1.ConditionFalse,
							Severity: clusterv1.ConditionSeverityError,
							Reason:   infrastructurev1beta1.BootstrapFailedReason,
							Message:  err.Error(),
						})
					}
					if exit {
						log.Info("Exiting program after bootstrap application.")
						return nil
					}
					log.Debugf("Waiting %s...", conf.Agent.Reconciliation.String())
					time.Sleep(conf.Agent.Reconciliation)
					continue
				}

				log.Debugf("Waiting %s...", conf.Agent.Reconciliation.String())
				time.Sleep(conf.Agent.Reconciliation)
			}
		},
	}

	//Define flags
	cmd.PersistentFlags().BoolVar(&versionFlag, "version", false, "print version and exit")
	cmd.PersistentFlags().BoolVar(&resetFlag, "reset", false, "reset the Elemental installation")
	cmd.PersistentFlags().BoolVar(&installFlag, "install", false, "install Elemental")
	cmd.PersistentFlags().BoolVar(&registerFlag, "register", false, "register Elemental host")
	cmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug logging")
	cmd.PersistentFlags().StringVar(&configPath, "config", configPathDefault, "agent config path")
	return cmd
}

func getConfig(fs vfs.FS) (config.Config, error) {
	conf := config.DefaultConfig()

	// Use go-vfs afero compatibility layer (required by Viper)
	afs := vfsafero.NewAferoFS(fs)
	viper.SetFs(afs)

	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		return config.Config{}, fmt.Errorf("reading config: %w", err)
	}

	if err := viper.Unmarshal(&conf); err != nil {
		return config.Config{}, fmt.Errorf("unmarshalling config: %w", err)
	}

	return conf, nil
}

func handleRegistration(client client.Client, osPlugin osplugin.Plugin, pubKey []byte, registrationToken string, registrationRecoveryPeriod time.Duration) (string, *api.RegistrationResponse) {
	hostnameFormatter := hostname.NewFormatter(osPlugin)
	var newHostname string
	var registration *api.RegistrationResponse
	var err error
	registrationError := false
	for {
		// Wait for recovery
		if registrationError {
			log.Debugf("Waiting '%s' on registration error to recover", registrationRecoveryPeriod)
			time.Sleep(registrationRecoveryPeriod)
		}
		// Fetch remote Registration
		log.Debug("Fetching remote registration")
		registration, err = client.GetRegistration(registrationToken)
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
		if err := client.CreateHost(api.HostCreateRequest{
			Name:        newHostname,
			Annotations: registration.HostAnnotations,
			Labels:      registration.HostLabels,
			PubKey:      string(pubKey),
		}, registrationToken); err != nil {
			log.Error(err, "registering new ElementalHost")
			registrationError = true
			continue
		}
		break
	}
	return newHostname, registration
}

func handlePostRegistration(osPlugin osplugin.Plugin, hostnameToSet string, id identity.Identity, registration *api.RegistrationResponse) error {
	// Persist registered hostname
	if err := osPlugin.InstallHostname(hostnameToSet); err != nil {
		return fmt.Errorf("persisting hostname '%s': %w", hostnameToSet, err)
	}
	// Persist agent config
	agentConfig := config.FromAPI(registration)
	agentConfigBytes, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("marshalling agent config: %w", err)
	}
	if err := osPlugin.InstallFile(agentConfigBytes, configPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting agent config file '%s': %w", configPath, err)
	}
	// Persist identity file
	identityBytes, err := id.Marshal()
	if err != nil {
		return fmt.Errorf("marshalling identity: %w", err)
	}
	privateKeyPath := fmt.Sprintf("%s/%s", agentConfig.Agent.WorkDir, identity.PrivateKeyFile)
	if err := osPlugin.InstallFile(identityBytes, privateKeyPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting private key file '%s': %w", privateKeyPath, err)
	}
	return nil
}

func handleInstall(client client.Client, osPlugin osplugin.Plugin, hostname string, registrationToken string, installationRecoveryPeriod time.Duration) {
	cloudConfigAlreadyApplied := false
	alreadyInstalled := false
	var installationError error
	installationErrorReason := infrastructurev1beta1.InstallationFailedReason
	for {
		if installationError != nil {
			// Log error
			log.Error(installationError, "installing host")
			// Attempt to report failed condition on management server
			attemptConditionReporting(client, hostname, clusterv1.Condition{
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
			log.Debugf("Waiting '%s' on installation error for installation instructions to mutate", installationRecoveryPeriod)
			time.Sleep(installationRecoveryPeriod)
		}
		// Fetch remote Registration
		var registration *api.RegistrationResponse
		var err error
		if !cloudConfigAlreadyApplied || !alreadyInstalled {
			log.Debug("Fetching remote registration")
			registration, err = client.GetRegistration(registrationToken)
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
			if err := osPlugin.InstallCloudInit(cloudConfigBytes); err != nil {
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
			if err := osPlugin.Install(installBytes); err != nil {
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
		if _, err := client.PatchHost(patchRequest, hostname); err != nil {
			installationError = fmt.Errorf("patching host with installation successful: %w", err)
			continue
		}
		break
	}
}

func handleReset(client client.Client, osPlugin osplugin.Plugin, hostname string, registrationToken string, resetRecoveryPeriod time.Duration) {
	var resetError error
	alreadyReset := false
	for {
		// Wait for recovery (end user may fix the remote reset instructions meanwhile)
		if resetError != nil {
			// Log error
			log.Error(resetError, "resetting")
			// Attempt to report failed condition on management server
			attemptConditionReporting(client, hostname, clusterv1.Condition{
				Type:     infrastructurev1beta1.ResetReady,
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   infrastructurev1beta1.ResetFailedReason,
				Message:  resetError.Error(),
			})
			// Clear error for next attempt
			resetError = nil
			log.Debugf("Waiting '%s' on reset error for reset instructions to mutate", resetRecoveryPeriod)
			time.Sleep(resetRecoveryPeriod)
		}
		// Mark ElementalHost for deletion
		// Repeat in case of failures. May be exploited server side to track repeated attempts.
		log.Debugf("Marking ElementalHost for deletion: %s", hostname)
		if err := client.DeleteHost(hostname); err != nil {
			resetError = fmt.Errorf("marking host for deletion: %w", err)
			continue
		}
		// Reset
		if !alreadyReset {
			// Fetch remote Registration
			log.Debug("Fetching remote registration")
			registration, err := client.GetRegistration(registrationToken)
			if err != nil {
				resetError = fmt.Errorf("getting remote Registration: %w", err)
				continue
			}
			log.Debug("Resetting...")
			resetBytes, err := json.Marshal(registration.Config.Elemental.Reset)
			if err != nil {
				resetError = fmt.Errorf("marshalling reset config: %w", err)
				continue
			}
			if err := osPlugin.Reset(resetBytes); err != nil {
				resetError = fmt.Errorf("resetting host: %w", err)
				continue
			}
			alreadyReset = true
		}
		// Report reset success
		log.Debug("Patching ElementalHost as reset")
		patchRequest := api.HostPatchRequest{Reset: ptr.To(true)}
		patchRequest.SetCondition(infrastructurev1beta1.ResetReady,
			corev1.ConditionTrue,
			clusterv1.ConditionSeverityInfo,
			"", "")
		if _, err := client.PatchHost(patchRequest, hostname); err != nil {
			resetError = fmt.Errorf("patching host with reset successful: %w", err)
			continue
		}
		break
	}
}

// handleBootstrap is usually called twice during the bootstrap phase.
//
// The first call should normally fetch the remote bootstrap config and propagate it to the plugin implementation.
// The system should then reboot, and upon successful reboot, the `/run/cluster-api/bootstrap-success.complete`
// sentinel file is expected to exist.
// Note that the reboot is currently enforced, since both `cloud-init` and `ignition` formats are meant to be applied
// during system boot.
// See: https://cluster-api.sigs.k8s.io/developer/providers/bootstrap.html#sentinel-file
//
// The second call should normally patch the remote Host resource as bootstrapped,
// after verifying the existance of `/run/cluster-api/bootstrap-success.complete`.
// Note that since `/run` is normally mounted as tmpfs and the bootstrap config is not re-executed at every boot,
// the remote host needs to be patched before the system is ever rebooted an additional time.
// If reboot happens and `/run/cluster-api/bootstrap-success.complete` is not found on the already-bootstrapped system,
// the plugin will be invoked again to re-apply the bootstrap config. It's up to the plugin implementation to recover
// from this state if possible, or to just return an error to highlight manual intervention is needed (and possibly a machine reset).
func handleBootstrap(fs vfs.FS, client client.Client, osPlugin osplugin.Plugin, hostname string) (bool, error) {
	// Assume system was already bootstrapped if sentinel file is found
	_, err := fs.Stat(bootstrapSentinelFile)
	if err == nil {
		// Patch the ElementalHost as successfully bootstrapped
		patchRequest := api.HostPatchRequest{Bootstrapped: ptr.To(true)}
		patchRequest.SetCondition(infrastructurev1beta1.BootstrapReady,
			corev1.ConditionTrue,
			clusterv1.ConditionSeverityInfo,
			"", "")
		if _, err := client.PatchHost(patchRequest, hostname); err != nil {
			return false, fmt.Errorf("patching ElementalHost after bootstrap: %w", err)
		}
		log.Info("Bootstrap config applied successfully")
		return false, nil
	}

	// Sentinel file not found, assume system needs bootstrapping
	if os.IsNotExist(err) {
		log.Debug("Fetching bootstrap config")
		bootstrap, err := client.GetBootstrap(hostname)
		if err != nil {
			return false, fmt.Errorf("fetching bootstrap config: %w", err)
		}
		log.Info("Applying bootstrap config")
		if err := osPlugin.Bootstrap(bootstrap.Format, []byte(bootstrap.Config)); err != nil {
			return false, fmt.Errorf("applying bootstrap config: %w", err)
		}
		attemptConditionReporting(client, hostname, clusterv1.Condition{
			Type:     infrastructurev1beta1.BootstrapReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1beta1.WaitingForBootstrapReasonSeverity,
			Reason:   infrastructurev1beta1.WaitingForBootstrapReason,
			Message:  "Waiting for bootstrap to be executed",
		})
		log.Info("System is rebooting to execute the bootstrap configuration...")
		if err := osPlugin.Reboot(); err != nil {
			// Exit the program in case of reboot failures
			// Assume this is not recoverable and requires manual intervention
			return true, fmt.Errorf("rebooting system for bootstrapping: %w", err)
		}
		return true, nil
	}

	return false, fmt.Errorf("verifying bootstrap sentinel file '%s': %w", bootstrapSentinelFile, err)
}

func handlePost(osPlugin osplugin.Plugin, poweroff bool, reboot bool) {
	if poweroff {
		log.Info("Powering off system")
		if err := osPlugin.PowerOff(); err != nil {
			log.Error(err, "Powering off system")
		}
	} else if reboot {
		log.Info("Rebooting system")
		if err := osPlugin.Reboot(); err != nil {
			log.Error(err, "Rebooting system")
		}
	}
}

// attemptConditionReporting is a best effort method to update the remote condition.
// Due to the unexpected nature of failures, we should not attempt indefinitely as there is no indication for recovery.
// For example if a network error occurs, leading to a failed condition, it's likely that reporting the condition will fail as well.
// The controller should always try to reconcile the 'True' status for each Host condition, so reporting failures should not be critical.
func attemptConditionReporting(client client.Client, hostname string, condition clusterv1.Condition) {
	if _, err := client.PatchHost(api.HostPatchRequest{
		Condition: &condition,
	}, hostname); err != nil {
		log.Error(err, "reporting condition", "conditionType", condition.Type, "conditionReason", condition.Reason)
	}
}
