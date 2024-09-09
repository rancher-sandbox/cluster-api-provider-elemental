package agent

import (
	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
)

// handlePost handles post conditions such as Reboot or PowerOff.
// A true flag is returned if any of the conditions is true, to highlight the program should exit.
func handlePost(osPlugin osplugin.Plugin, post infrastructurev1.PostAction) bool {
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
