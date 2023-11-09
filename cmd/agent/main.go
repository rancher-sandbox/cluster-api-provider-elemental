package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/identity"
	log "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfsafero"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/ptr"
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
	commandRunner := utils.NewCommandRunner()
	cmd := newCommand(fs, osPluginLoader, commandRunner, client)
	if err := cmd.Execute(); err != nil {
		log.Error(err, "running elemental-agent")
		os.Exit(1)
	}
}

func newCommand(fs vfs.FS, pluginLoader osplugin.Loader, commandRunner utils.CommandRunner, client client.Client) *cobra.Command {
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
				hostname, registration = handleRegistration(client, osPlugin, pubKey, conf.Agent.Reconciliation)
				log.Infof("Successfully registered as '%s'", hostname)
				if err := handlePostRegistration(osPlugin, hostname, identity, registration); err != nil {
					return fmt.Errorf("handling post registration: %w", err)
				}
				// Exit program if --install was not called
				if !installFlag {
					return nil
				}
			}
			// Install
			if installFlag {
				log.Info("Installing Elemental")
				handleInstall(client, osPlugin, hostname, conf.Agent.Reconciliation)
				log.Info("Installation successful")
				handlePost(osPlugin, conf.Agent.PostInstall.PowerOff, conf.Agent.PostInstall.Reboot)
				return nil
			}

			// Reset
			if resetFlag {
				log.Info("Resetting Elemental")
				handleReset(client, osPlugin, conf.Agent.Reconciliation, hostname)
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

				// Handle bootstrap if needed
				if host.BootstrapReady && !host.Bootstrapped {
					log.Info("Applying bootstrap config")
					if err := handleBootstrap(commandRunner, fs, client, hostname); err != nil {
						log.Error(err, "handling bootstrap")
						log.Debugf("Waiting %s...", conf.Agent.Reconciliation.String())
						time.Sleep(conf.Agent.Reconciliation)
						continue
					}
					log.Info("Bootstrap config applied successfully")
				}

				// Handle Reset Needed
				if host.NeedsReset {
					log.Info("Triggering reset")
					if err := osPlugin.TriggerReset(); err != nil {
						log.Error(err, "handling reset needed")
					} else {
						// If Reset was triggered successfully, exit the program.
						log.Info("Reset was triggered successfully. Exiting program.")
						return nil
					}
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

func handleRegistration(client client.Client, osPlugin osplugin.Plugin, pubKey []byte, registrationRecoveryPeriod time.Duration) (string, *api.RegistrationResponse) {
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
		registration, err = client.GetRegistration()
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
		}); err != nil {
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
	if err := osPlugin.PersistHostname(hostnameToSet); err != nil {
		return fmt.Errorf("persisting hostname '%s': %w", hostnameToSet, err)
	}
	// Persist agent config
	agentConfig := config.FromAPI(registration)
	agentConfigBytes, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("marshalling agent config: %w", err)
	}
	if err := osPlugin.PersistFile(agentConfigBytes, configPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting agent config file '%s': %w", configPath, err)
	}
	// Persist identity file
	identityBytes, err := id.Marshal()
	if err != nil {
		return fmt.Errorf("marshalling identity: %w", err)
	}
	privateKeyPath := fmt.Sprintf("%s/%s", agentConfig.Agent.WorkDir, identity.PrivateKeyFile)
	if err := osPlugin.PersistFile(identityBytes, privateKeyPath, 0640, 0, 0); err != nil {
		return fmt.Errorf("persisting private key file '%s': %w", privateKeyPath, err)
	}
	return nil
}

func handleInstall(client client.Client, osPlugin osplugin.Plugin, hostname string, installationRecoveryPeriod time.Duration) {
	cloudConfigAlreadyApplied := false
	alreadyInstalled := false
	installationError := false
	for {
		// Wait for recovery (end user may fix the remote installation instructions meanwhile)
		if installationError {
			log.Debugf("Waiting '%s' on installation error for installation instructions to mutate", installationRecoveryPeriod)
			time.Sleep(installationRecoveryPeriod)
		}
		// Fetch remote Registration
		var registration *api.RegistrationResponse
		var err error
		if !cloudConfigAlreadyApplied || !alreadyInstalled {
			log.Debug("Fetching remote registration")
			registration, err = client.GetRegistration()
			if err != nil {
				log.Error(err, "getting remote Registration")
				installationError = true
				continue
			}
		}
		// Apply Cloud Config
		if !cloudConfigAlreadyApplied {
			cloudConfigBytes, err := json.Marshal(registration.Config.CloudConfig)
			if err != nil {
				log.Error(err, "marshalling cloud config")
				installationError = true
				continue
			}
			if err := osPlugin.ApplyCloudInit(cloudConfigBytes); err != nil {
				log.Error(err, "applying cloud config")
				installationError = true
				continue
			}
			cloudConfigAlreadyApplied = true
		}
		// Install
		if !alreadyInstalled {
			installBytes, err := json.Marshal(registration.Config.Elemental.Install)
			if err != nil {
				log.Error(err, "marshalling install config")
				installationError = true
				continue
			}
			if err := osPlugin.Install(installBytes); err != nil {
				// TODO: Patch the Elemental Host with installation failure status and reason
				log.Error(err, "installing Elemental")
				installationError = true
				continue
			}
			alreadyInstalled = true
		}
		// Report installation success
		if _, err := client.PatchHost(api.HostPatchRequest{
			Installed: ptr.To(true),
		}, hostname); err != nil {
			log.Error(err, "patching host with installation successful")
			installationError = true
			continue
		}
		break
	}
}

func handleReset(client client.Client, osPlugin osplugin.Plugin, resetRecoveryPeriod time.Duration, hostname string) {
	resetError := false
	alreadyReset := false
	for {
		// Wait for recovery (end user may fix the remote reset instructions meanwhile)
		if resetError {
			log.Debugf("Waiting '%s' on reset error for reset instructions to mutate", resetRecoveryPeriod)
			time.Sleep(resetRecoveryPeriod)
		}
		// Mark ElementalHost for deletion
		// Repeat in case of failures. May be exploited server side to track repeated attempts.
		log.Debugf("Marking ElementalHost for deletion: %s", hostname)
		if err := client.DeleteHost(hostname); err != nil {
			log.Error(err, "marking host for deletion")
			resetError = true
			continue
		}
		// Reset
		if !alreadyReset {
			// Fetch remote Registration
			log.Debug("Fetching remote registration")
			registration, err := client.GetRegistration()
			if err != nil {
				log.Error(err, "getting remote Registration")
				resetError = true
				continue
			}
			log.Debug("Resetting...")
			resetBytes, err := json.Marshal(registration.Config.Elemental.Reset)
			if err != nil {
				log.Error(err, "marshalling reset config")
				resetError = true
				continue
			}
			if err := osPlugin.Reset(resetBytes); err != nil {
				// TODO: Patch the Elemental Host with reset failure status and reason
				log.Error(err, "resetting Elemental")
				resetError = true
				continue
			}
			alreadyReset = true
		}
		// Report reset success
		log.Debug("Patching ElementalHost as reset")
		if _, err := client.PatchHost(api.HostPatchRequest{
			Reset: ptr.To(true),
		}, hostname); err != nil {
			log.Error(err, "patching host with reset successful")
			resetError = true
			continue
		}
		break
	}
}

func handleBootstrap(cmdRunner utils.CommandRunner, fs vfs.FS, client client.Client, hostname string) error {
	// Avoid applying bootstrap multiple times
	// See contract: https://cluster-api.sigs.k8s.io/developer/providers/bootstrap.html#sentinel-file
	_, err := fs.Stat(bootstrapSentinelFile)
	if os.IsNotExist(err) {
		log.Debug("Fetching bootstrap config")
		bootstrap, err := client.GetBootstrap(hostname)
		if err != nil {
			return fmt.Errorf("fetching bootstrap config: %w", err)
		}

		for _, file := range bootstrap.Files {
			if err := utils.WriteFile(fs, file); err != nil {
				return fmt.Errorf("writing bootstrap file: %w", err)
			}
		}

		for _, command := range bootstrap.Commands {
			if err := cmdRunner.RunCommand(command); err != nil {
				return fmt.Errorf("running bootstrap command: %w", err)
			}
		}
	} else if err != nil {
		return fmt.Errorf("verifying bootstrap sentinel file '%s': %w", bootstrapSentinelFile, err)
	}

	// Patch the ElementalHost as successfully bootstrapped
	if _, err := client.PatchHost(api.HostPatchRequest{Bootstrapped: ptr.To(true)}, hostname); err != nil {
		return fmt.Errorf("patching ElementalHost after bootstrap: %w", err)
	}
	log.Info("Host successfully patched as bootstrapped")

	return nil
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
