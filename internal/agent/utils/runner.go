package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
)

// RunCommand runs the input command through bash.
// This implies `/bin/bash` is installed on the host.
func RunCommand(command string) error {
	log.Infof("Running command: %s", command)
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "-c", command)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running command: %w", err)
	}
	return nil
}
