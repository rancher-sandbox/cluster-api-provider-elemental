package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfsafero/v4"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	log "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase/phases"
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
	phaseHandler := phase.NewHostPhaseHandler()
	cmd := newCommand(fs, osPluginLoader, client, phaseHandler)
	if err := cmd.Execute(); err != nil {
		log.Error(err, "running elemental-agent")
		os.Exit(1)
	}
}

func newCommand(fs vfs.FS, pluginLoader osplugin.Loader, client client.Client, phaseHandler phase.HostPhaseHandler) *cobra.Command {
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
			// Initialize phase handler
			hostContext := phase.HostContext{
				AgentConfig:     conf,
				AgentConfigPath: configPath,
				Hostname:        hostname,
			}
			phaseHandler.Init(fs, client, osPlugin, identity, hostContext)

			// Register
			if registerFlag {
				log.Info("Registering Elemental Host")
				_, err := phaseHandler.Handle(infrastructurev1beta1.PhaseRegistering)
				if err != nil {
					return fmt.Errorf("handling registration: %w", err)
				}
				log.Info("Finalizing Registration")
				_, err = phaseHandler.Handle(infrastructurev1beta1.PhaseFinalizingRegistration)
				if err != nil {
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
				post, err := phaseHandler.Handle(infrastructurev1beta1.PhaseInstalling)
				if err != nil {
					return fmt.Errorf("handling install: %w", err)
				}
				log.Info("Installation successful")
				handlePost(osPlugin, post)
				return nil
			}

			// Reset
			if resetFlag {
				log.Info("Resetting Elemental")
				post, err := phaseHandler.Handle(infrastructurev1beta1.PhaseResetting)
				if err != nil {
					return fmt.Errorf("handling reset: %w", err)
				}
				log.Info("Reset successful")
				handlePost(osPlugin, post)
				return nil
			}

			// Normal reconcile
			log.Info("Entering reconciliation loop")
			runningPhase := infrastructurev1beta1.PhaseRunning
			for {
				// Patch the host and receive the patched remote host back
				log.Debug("Patching host")
				host, err := client.PatchHost(api.HostPatchRequest{
					Phase: &runningPhase,
				}, hostname)
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
					_, err := phaseHandler.Handle(infrastructurev1beta1.PhaseTriggeringReset)
					if err != nil {
						log.Error(err, "handling reset trigger")
						log.Debugf("Waiting %s...", conf.Agent.Reconciliation.String())
						time.Sleep(conf.Agent.Reconciliation)
						continue
					}
					// If Reset was triggered successfully, exit the program.
					log.Info("Reset was triggered successfully. Exiting program.")
					return nil
				}

				// Handle bootstrap if needed
				if host.BootstrapReady && !host.Bootstrapped {
					log.Info("Handling bootstrap application")
					post, err := phaseHandler.Handle(infrastructurev1beta1.PhaseBootstrapping)
					if err != nil {
						log.Error(err, "handling bootstrap")
						log.Debugf("Waiting %s...", conf.Agent.Reconciliation.String())
						time.Sleep(conf.Agent.Reconciliation)
						continue
					}
					if handlePost(osPlugin, post) {
						// Exit the program if we are rebooting to apply bootstrap
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

// handlePost handles post conditions such as Reboot or PowerOff.
// A true flag is returned if any of the conditions is true, to highlight the program should exit.
func handlePost(osPlugin osplugin.Plugin, post phases.PostCondition) bool {
	if post.PowerOff {
		log.Info("Powering off system")
		if err := osPlugin.PowerOff(); err != nil {
			log.Error(err, "Powering off system")
		}
		return true
	} else if post.Reboot {
		log.Info("Rebooting system")
		if err := osPlugin.Reboot(); err != nil {
			log.Error(err, "Rebooting system")
		}
		return true
	}
	return false
}
