package agent

import (
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase"
	"github.com/spf13/cobra"
)

// resetCmd represents the reset command.
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Resets this Elemental host",
	Long:  "Resets this Elemental host",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Initializing agent")
		agentContext, err := InitAgent()
		if err != nil {
			log.Fatal(err, "Could not initialize agent")
		}

		log.Info("Resetting host")
		resetHandler := phase.NewResetHandler(*agentContext)
		resetHandler.Reset()
		if handlePost(agentContext.Plugin, agentContext.Config.Agent.PostReset) {
			// Program should exit
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
