package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	log "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/hostname"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfsafero"
	"k8s.io/utils/ptr"
)

const (
	configPathDefault = "/oem/elemental/agent/config.yaml"
)

// Flags.
var (
	versionFlag bool
	resetFlag   bool
	installFlag bool
	configPath  string
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
			config, err := getConfig(fs)
			if err != nil {
				return fmt.Errorf("parsing configuration file '%s': %w", configPath, err)
			}
			// Set debug logs
			if config.Agent.Debug {
				log.EnableDebug()
				log.Debug("Debug logging enabled")
			}
			// Initialize Elemental API Client
			client, err := client.NewClient(fs, config)
			if err != nil {
				return fmt.Errorf("initializing Elemental API client: %w", err)
			}
			if installFlag {
				log.Info("Installing Elemental")
				// This should:
				// 1. Get the remote ElementalRegistration
				//
				// client.GetRegistration()
				//
				// 2. Pick and set a Hostname according to the remote ElementalRegistration config
				//	  Still not sure about this. The problem is that the Hostname is used as primary key in the HTTP api.
				//    This may lead to collisions, so then what should the end user do in this case?
				//    Do we want to error out and force a reprovisioning in case 2 machines within the same registration have the same hostname?
				//    If not we can change the primary keys to use the k8s UUIDs instead.
				//    Note that in any case the elemental-toolkit will need to persist some hostnamectl instructions on install and reset.
				//
				// if registration.hostname.useExisting {
				//	 hostname = hostname.FormatCurrent() // The idea here is that this one may be set by the DHCP client
				// } else {
				//   hostname = hostname.FormatRandom()
				// }
				//
				// 3. Create new private/pub keys pair
				//
				// tls.yadda yadda
				//
				// 4. POST a new ElementalHost through the API (including the pub key to be used to authorize further PATCH request)
				//
				// client.CreateMachineHost(mynewhost)
				//
				// 4. Install Elemental
				//    Considering points 2 and 3, we will need elemental-toolkit to persist the hostname and the key pair.
				//    This can already be done simply by exploting cloud-init, no changes needed.
				//
				// installer.InstallElemental()
				//
				// 5. PATCH the ElementalHost with the "installed" flag on
				//    Note that we first register the ElementalHost and then we attempt the installation.
				//    This has the benefit of enabling tracking of the installation status, but what to do if the installation goes wrong?
				//    Ideally the agent will try to recover by only repeating step 1. and 4., to fetch a potentially updated registration and try install again.
				//    However if this ultimately fails, maybe because the hardware is found to be defective beyond repair, the end user will need to clean the ElementalHost manually.
				//
				// client.PatchMachineHost(myhostpatch) // --> "installed": true
				//
				// 5. Reboot to active system
				return nil
			}
			if resetFlag {
				log.Info("Resetting Elemental")
				// Very similar to install flow
				return nil
			}

			// <JustForDemo>
			var hname string
			log.Info("Demoing Elemental")
			registration, err := client.GetRegistration()
			if err != nil {
				return fmt.Errorf("getting remote registration: %w", err)
			}

			log.Info("Setting hostname")
			if registration.Config.Elemental.Registration.Hostname.UseExisting {
				hname, err = hostname.FormatCurrent(registration.Config.Elemental.Registration.Hostname.Prefix)
				if err != nil {
					return fmt.Errorf("formatting current hostname with prefix '%s': %w", registration.Config.Elemental.Registration.Hostname.Prefix, err)
				}
			} else {
				hname = hostname.FormatRandom(registration.Config.Elemental.Registration.Hostname.Prefix)
			}
			log.Infof("Picked hostname '%s'", hname)

			log.Info("Registering new ElementalHost")
			if err := client.CreateMachineHost(api.HostCreateRequest{
				Name: hname,
			}); err != nil {
				return fmt.Errorf("registering new ElementalHost: %w", err)
			}

			log.Info("Pretending that the installation was successful")
			if _, err := client.PatchMachineHost(api.HostPatchRequest{
				Installed: ptr.To(true),
			}, hname); err != nil {
				return fmt.Errorf("patching ElementalHost after installation: %w", err)
			}
			// </JustForDemo>

			// Main host reconciliation loop
			for { // TODO: Maybe use os signals to exit from here nicely
				log.Info("Entering reconciliation loop")

				host, err := client.PatchMachineHost(api.HostPatchRequest{}, hname)
				if err != nil {
					log.Error(fmt.Errorf("patching ElementalHost during normal reconcile: %w", err), "")
				}

				if host.BootstrapReady && !host.Bootstrapped {
					log.Info("Fetching bootstrap instructions")
					bootstrap, err := client.GetBootstrap(hname)
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
					if _, err := client.PatchMachineHost(api.HostPatchRequest{Bootstrapped: ptr.To(true)}, hname); err != nil {
						log.Error(fmt.Errorf("patching ElementalHost after bootstrap: %w", err), "")
					}
				}

				time.Sleep(config.Agent.Reconciliation)
			}
		},
	}

	//Define flags
	cmd.PersistentFlags().BoolVar(&versionFlag, "version", false, "print version and exit")
	cmd.PersistentFlags().BoolVar(&resetFlag, "reset", false, "reset the Elemental installation")
	cmd.PersistentFlags().BoolVar(&installFlag, "install", false, "install Elemental")
	cmd.PersistentFlags().StringVar(&configPath, "config", configPathDefault, "agent config path")
	return cmd
}

func getConfig(fs vfs.FS) (agent.Config, error) {
	config := agent.DefaultConfig()

	// Use go-vfs afero compatibility layer (required by Viper)
	afs := vfsafero.NewAferoFS(fs)
	viper.SetFs(afs)

	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		return agent.Config{}, fmt.Errorf("reading config: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return agent.Config{}, fmt.Errorf("unmarshalling config: %w", err)
	}

	return config, nil
}
