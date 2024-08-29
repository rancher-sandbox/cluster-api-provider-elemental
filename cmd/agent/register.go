package agent

import (
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase"
	"github.com/spf13/cobra"
)

var (
	installFlag bool
)

// registerCmd represents the register command.
var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Registers this Elemental host to the remote CAPI management cluster",
	Long:  "Registers this Elemental host to the remote CAPI management cluster",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Initializing agent")
		agentContext, err := InitAgent()
		if err != nil {
			log.Fatal(err, "Could not initialize agent")
		}
		registrationHandler := phase.NewRegistrationHandler(agentContext)

		log.Info("Registering new host")
		if err := registrationHandler.Register(); err != nil {
			log.Fatal(err, "Could not register host")
		}
		log.Info("Finalizing Registration")
		if err := registrationHandler.FinalizeRegistration(); err != nil {
			log.Fatal(err, "Could not finalize registration")
		}

		// Directly install host if requested
		if installFlag {
			log.Info("Installing host")
			installationHandler := phase.NewInstallHandler(*agentContext)
			installationHandler.Install()
			if handlePost(agentContext.Plugin, agentContext.Config.Agent.PostInstall) {
				// Program should exit
				return
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)

	registerCmd.Flags().BoolVar(&installFlag, "install", false, "Automatically installs the system after registration")
}
