package main

import (
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
	configPathDefault = "/etc/elemental/agent/config.yaml"
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

func main() {
	fs := vfs.OSFS
	cmd := newCommand(fs)
	if err := cmd.Execute(); err != nil {
		log.Error(err, "running elemental-agent")
		os.Exit(1)
	}
}

func newCommand(fs vfs.FS) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "elemental-agent",
		Short: "Elemental Agent command",
		Long:  "elemental-agent registers a node with the elemental-operator via a config file",
		RunE: func(_ *cobra.Command, args []string) error {
			// Display version
			if versionFlag {
				log.Infof("Register version %s, commit %s, commit date %s", version.Version, version.Commit, version.CommitDate)
				return nil
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
			// Sanity checks
			if installFlag && resetFlag {
				log.Info("--install and --reset are mutually exclusive")
				return nil
			}
			// Initialize WorkDir
			if err := utils.CreateDirectory(fs, conf.Agent.WorkDir); err != nil {
				return fmt.Errorf("creating work directory '%s': %w", conf.Agent.WorkDir, err)
			}
			// Initialize Elemental API Client
			client, err := client.NewClient(fs, conf)
			if err != nil {
				return fmt.Errorf("initializing Elemental API client: %w", err)
			}
			// Fetch remote Registration
			registration, err := client.GetRegistration()
			if err != nil {
				return fmt.Errorf("getting remote Registration: %w", err)
			}
			// Get current hostname
			currentHostname, err := hostname.GetCurrentHostname()
			if err != nil {
				return fmt.Errorf("getting current hostname: %w", err)
			}
			// Initialize installed (also needed to trigger host reset)
			log.Info("Initializing Installer")
			var installer host.Installer
			if conf.Agent.OSNotManaged {
				log.Info("Using Unmanaged OS Installer")
				installer = host.NewUnmanagedInstaller(fs, conf.Agent.WorkDir)
			} else {
				log.Info("Using Elemental Installer")
				installer = host.NewElementalInstaller(fs)
			}

			// Install
			if installFlag {
				log.Info("Installing Elemental")
				// Pick the new hostname
				newHostname, err := hostname.PickHostname(registration.Config.Elemental.Agent.Hostname)
				if err != nil {
					return fmt.Errorf("picking new hostname: %w", err)
				}
				// Register new Elemental Host
				if err := client.CreateHost(api.HostCreateRequest{
					Name:        newHostname,
					Annotations: registration.HostAnnotations,
					Labels:      registration.HostLabels,
				}); err != nil {
					return fmt.Errorf("registering new ElementalHost: %w", err)
				}
				// Install
				if err := installer.Install(registration, newHostname); err != nil {
					return fmt.Errorf("installing Elemental: %w", err)
				}
				// Report installation success
				if _, err := client.PatchHost(api.HostPatchRequest{
					Installed: ptr.To(true),
				}, newHostname); err != nil {
					return fmt.Errorf("patching host with installation successful: %w", err)
				}
				return nil
			}

			// Reset
			if resetFlag {
				log.Info("Resetting Elemental")
				// Mark ElementalHost for deletion
				if err := client.DeleteHost(currentHostname); err != nil {
					return fmt.Errorf("marking host for deletion")
				}
				// Reset
				if err := installer.Reset(registration); err != nil {
					return fmt.Errorf("resetting Elemental: %w", err)
				}
				// Report reset success
				if _, err := client.PatchHost(api.HostPatchRequest{
					Reset: ptr.To(true),
				}, currentHostname); err != nil {
					return fmt.Errorf("patching host with reset successfull: %w", err)
				}
				return nil
			}

			for {
				log.Info("Entering reconciliation loop")

				// Patch the host and receive the patched remote host back
				host, err := client.PatchHost(api.HostPatchRequest{}, currentHostname)
				if err != nil {
					log.Error(fmt.Errorf("patching ElementalHost during normal reconcile: %w", err), "")
				}

				// Handle bootstrap if needed
				if host.BootstrapReady && !host.Bootstrapped {
					log.Info("Fetching bootstrap instructions")
					bootstrap, err := client.GetBootstrap(currentHostname)
					if err != nil {
						log.Error(fmt.Errorf("fetching bootstrap instructions: %w", err), "")
					}

					for _, file := range bootstrap.Files {
						if err := utils.WriteFile(fs, file); err != nil {
							log.Error(err, "writing bootstrap file")
						}
					}

					for _, command := range bootstrap.Commands {
						if err := utils.RunCommand(command); err != nil {
							log.Error(err, "running bootstrap command")
						}
					}

					log.Info("Applied bootstrap instructions")
					if _, err := client.PatchHost(api.HostPatchRequest{Bootstrapped: ptr.To(true)}, currentHostname); err != nil {
						log.Error(fmt.Errorf("patching ElementalHost after bootstrap: %w", err), "")
					}
				}

				// Handle Reset Needed
				if host.NeedsReset {
					if err := installer.TriggerReset(registration); err != nil {
						log.Error(fmt.Errorf("handling reset needed: %w", err), "")
					}
					// If Reset was triggered successfully, exit the program.
					return nil
				}

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
	conf := config.Config{}

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
