package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname"
	log "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfsafero"
	"k8s.io/utils/ptr"
)

const (
	configPathDefault     = "/etc/elemental/agent/config.yaml"
	bootstrapSentinelFile = "/run/cluster-api/bootstrap-success.complete"
)

// Flags.
var (
	versionFlag bool
	resetFlag   bool
	installFlag bool
	debugFlag   bool
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
	installerSelector := host.NewInstallerSelector()
	hostnameManager := hostname.NewManager()
	client := client.NewClient()
	cmd := newCommand(fs, installerSelector, hostnameManager, client)
	if err := cmd.Execute(); err != nil {
		log.Error(err, "running elemental-agent")
		os.Exit(1)
	}
}

func newCommand(fs vfs.FS, installerSelector host.InstallerSelector, hostnameManager hostname.Manager, client client.Client) *cobra.Command {
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
			// Initialize Elemental API Client
			if err := client.Init(fs, conf); err != nil {
				return fmt.Errorf("initializing Elemental API client: %w", err)
			}
			// Get current hostname
			currentHostname, err := hostnameManager.GetCurrentHostname()
			if err != nil {
				return fmt.Errorf("getting current hostname: %w", err)
			}
			// Initialize installer
			log.Info("Initializing Installer")
			installer, err := installerSelector.GetInstaller(fs, configPath, conf)
			if err != nil {
				return fmt.Errorf("initializing installer: %w", err)
			}

			// Install
			if installFlag {
				log.Info("Installing Elemental")
				handleInstall(client, hostnameManager, installer, conf.Agent.Reconciliation)
				log.Info("Installation successful")
				return nil
			}

			// Reset
			if resetFlag {
				log.Info("Resetting Elemental")
				handleReset(client, installer, conf.Agent.Reconciliation, currentHostname)
				log.Info("Reset successful")
				return nil
			}

			// Normal reconcile
			log.Info("Entering reconciliation loop")
			for {
				// Patch the host and receive the patched remote host back
				log.Debug("Patching host")
				host, err := client.PatchHost(api.HostPatchRequest{}, currentHostname)
				if err != nil {
					log.Error(err, "patching ElementalHost during normal reconcile")
				}

				// Handle bootstrap if needed
				if host.BootstrapReady && !host.Bootstrapped {
					log.Info("Applying bootstrap config")
					if err := handleBootstrap(fs, client, currentHostname); err != nil {
						log.Error(err, "handling bootstrap")
					}
					log.Info("Bootstrap config applied successfully")
				}

				// Handle Reset Needed
				if host.NeedsReset {
					log.Info("Triggering reset")
					if err := installer.TriggerReset(); err != nil {
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

// TODO: Would be wiser to decouple host registration/agent-configuration from installation.
// This could introduce a new --register flag, leaving the --install as optional (for unmanaged OS for example).
// However, consider that setting the hostname must be part of the registration workflow,
// so maybe decoupling would not be possible without a state/cache file where to store the hostname-to-be-set.
func handleInstall(client client.Client, hostnameManager hostname.Manager, installer host.Installer, installationRecoveryPeriod time.Duration) {
	alreadyRegistered := false
	installationError := false
	var newHostname string
	for {
		// Wait for recovery (end user may fix the remote installation instructions meanwhile)
		if installationError {
			log.Debugf("Waiting '%s' on installation error for installation instructions to mutate", installationRecoveryPeriod)
			time.Sleep(installationRecoveryPeriod)
		}
		// Fetch remote Registration
		log.Debug("Fetching remote registration")
		registration, err := client.GetRegistration()
		if err != nil {
			log.Error(err, "getting remote Registration")
			installationError = true
			continue
		}
		// Pick the new hostname if not done yet
		if len(newHostname) == 0 {
			newHostname, err = hostnameManager.PickHostname(registration.Config.Elemental.Agent.Hostname)
			log.Debugf("Selected hostname: %s", newHostname)
			if err != nil {
				log.Error(err, "picking new hostname")
				installationError = true
				continue
			}
		}
		// Register new Elemental Host
		if !alreadyRegistered {
			log.Debugf("Registering new host: %s", newHostname)
			if err := client.CreateHost(api.HostCreateRequest{
				Name:        newHostname,
				Annotations: registration.HostAnnotations,
				Labels:      registration.HostLabels,
			}); err != nil {
				log.Error(err, "registering new ElementalHost")
				installationError = true
				continue
			}
			alreadyRegistered = true
		}
		// Install
		if err := installer.Install(registration, newHostname); err != nil {
			// TODO: Patch the Elemental Host with installation failure status and reason
			log.Error(err, "installing Elemental")
			installationError = true
			continue
		}
		// Report installation success
		if _, err := client.PatchHost(api.HostPatchRequest{
			Installed: ptr.To(true),
		}, newHostname); err != nil {
			log.Error(err, "patching host with installation successful")
			installationError = true
			continue
		}
		break
	}
}

func handleReset(client client.Client, installer host.Installer, resetRecoveryPeriod time.Duration, hostname string) {
	resetError := false
	alreadyReset := false
	for {
		// Wait for recovery (end user may fix the remote reset instructions meanwhile)
		if resetError {
			log.Debugf("Waiting '%s' on reset error for reset instructions to mutate", resetRecoveryPeriod)
			time.Sleep(resetRecoveryPeriod)
		}
		// Fetch remote Registration
		log.Debug("Fetching remote registration")
		registration, err := client.GetRegistration()
		if err != nil {
			log.Error(err, "getting remote Registration")
			resetError = true
			continue
		}
		// Mark ElementalHost for deletion
		log.Debugf("Marking ElementalHost for deletion: %s", hostname)
		if err := client.DeleteHost(hostname); err != nil {
			log.Error(err, "marking host for deletion")
			resetError = true
			continue
		}
		// Reset
		if !alreadyReset {
			log.Debug("Resetting...")
			if err := installer.Reset(registration); err != nil {
				// TODO: Patch the Elemental Host with reset failure status and reason
				log.Error(err, "resetting Elemental: %w")
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

func handleBootstrap(fs vfs.FS, client client.Client, hostname string) error {
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
			if err := utils.RunCommand(command); err != nil {
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
