package agent

import (
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase"
	"github.com/spf13/cobra"
)

// installCmd represents the install command.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Installs the OS on this Elemental host",
	Long:  "Installs the OS on this Elemental host",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Initializing agent")
		agentContext, err := InitAgent()
		if err != nil {
			log.Fatal(err, "Could not initialize agent")
		}

		log.Info("Installing host")
		installationHandler := phase.NewInstallHandler(*agentContext)
		installationHandler.Install()
		if handlePost(agentContext.Plugin, agentContext.Config.Agent.PostInstall) {
			// Program should exit
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
