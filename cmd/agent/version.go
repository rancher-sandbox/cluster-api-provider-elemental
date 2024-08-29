package agent

import (
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Returns the version of the elemental-agent",
	Long:  "Returns the version of the elemental-agent",
	Run: func(_ *cobra.Command, _ []string) {
		log.Infof("Agent version %s, commit %s, commit date %s", version.Version, version.Commit, version.CommitDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
