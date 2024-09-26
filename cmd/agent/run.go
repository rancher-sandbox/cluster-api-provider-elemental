package agent

import (
	"time"

	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/spf13/cobra"
)

// runCmd represents the run command.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Operates this Elemental host according to the remote CAPI conditions",
	Long: `Operates this Elemental host according to the remote CAPI conditions: 

This is the normal running operation of an Elemental host. 
This command will reconcile the remote ElementalHost resource describing this host.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Initializing agent")
		agentContext, err := InitAgent()
		if err != nil {
			log.Fatal(err, "Could not initialize agent")
		}
		// Normal reconcile
		log.Info("Entering reconciliation loop")
		runningPhase := infrastructurev1.PhaseRunning
		for {
			// Patch the host and receive the patched remote host back
			log.Debug("Patching host")
			host, err := agentContext.Client.PatchHost(api.HostPatchRequest{
				Phase: &runningPhase,
			}, agentContext.Hostname)
			if err != nil {
				log.Error(err, "Could not patch ElementalHost during normal reconcile")
				log.Debugf("Waiting %s...", agentContext.Config.Agent.Reconciliation.String())
				time.Sleep(agentContext.Config.Agent.Reconciliation)
				continue
			}

			// Handle Reset trigger
			//
			// Reset should always be prioritized in the normal reconcile loop,
			// to allow reset of machines that are otherwise stuck in other phases,
			// like bootstrapping.
			if host.NeedsReset {
				log.Info("Triggering reset")
				resetHandler := phase.NewResetHandler(*agentContext)
				if err := resetHandler.TriggerReset(); err != nil {
					log.Error(err, "handling reset trigger")
					log.Debugf("Waiting %s...", agentContext.Config.Agent.Reconciliation.String())
					time.Sleep(agentContext.Config.Agent.Reconciliation)
					continue
				}
				// If Reset was triggered successfully, exit the program.
				log.Info("Reset was triggered successfully. Exiting program.")
				return
			}

			// Handle Upgrade
			needsInplaceUpdate := host.InPlaceUpgrade == infrastructurev1.InPlaceUpdatePending
			if !host.Bootstrapped || needsInplaceUpdate {
				log.Info("Reconciling OS Version")
				osVersionHandler := phase.NewOSVersionHandler(*agentContext)
				post, err := osVersionHandler.Reconcile(host.OSVersionManagement, needsInplaceUpdate)
				if err != nil {
					log.Error(err, "handling OS reconciliation")
					log.Debugf("Waiting %s...", agentContext.Config.Agent.Reconciliation.String())
					time.Sleep(agentContext.Config.Agent.Reconciliation)
					continue
				}
				if handlePost(agentContext.Plugin, post) {
					// Exit the program if we are rebooting to apply bootstrap
					return
				}
			}

			// Handle bootstrap if needed
			if host.BootstrapReady && !host.Bootstrapped {
				log.Info("Handling bootstrap application")
				bootstrapHandler := phase.NewBootstrapHandler(*agentContext)
				post, err := bootstrapHandler.Bootstrap()
				if err != nil {
					log.Error(err, "handling bootstrap")
					log.Debugf("Waiting %s...", agentContext.Config.Agent.Reconciliation.String())
					time.Sleep(agentContext.Config.Agent.Reconciliation)
					continue
				}
				if handlePost(agentContext.Plugin, post) {
					// Exit the program if we are rebooting to apply bootstrap
					return
				}
			}

			log.Debugf("Waiting %s...", agentContext.Config.Agent.Reconciliation.String())
			time.Sleep(agentContext.Config.Agent.Reconciliation)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
