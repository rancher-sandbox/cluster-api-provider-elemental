package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/twpayne/go-vfs/v4"
	"gopkg.in/yaml.v3"
)

// TODO: This is just for a PoC. This logic is mainly used to determine whether the elemental plugin
// should invoke `elemental upgrade` or not.
// This should be implemented on the elemental-toolkit side, and use the already existing
// toolkit-driven state file as a contract that the plugin can consume. The plugin does not necessarily need to
// be stateless, but it would be best to maintain the state on one side only.
//
// Ideally the plugin could pass an upgrade identifier:
// 'elemental upgrade --id my_upgrade_plan_hash --system.uri my-image-uri'
//
// The identifier could be reflected in the elemental install state file, and finaly verified during boot assessment.
// In case of post-upgrade error, the identifier could be marked as failed instead, to prevent further attempts.
type ElementalInstallState struct {
	// LastAppliedURI can be used to determine if an upgrade needs to be triggered or not.
	LastAppliedURI string `yaml:"lastAppliedURI" mapstructure:"lastAppliedURI"`
	// LastOSReleaseHash is /etc/os-release hash. This is used to determine if we booted into the 'upgraded' system or not.
	LastOSReleaseHash string `yaml:"lastOSReleaseHash" mapstructure:"lastOSReleaseHash"`
}

var ErrUpgradeFailed = errors.New("Upgrade failed")

const (
	OSReleasePath = "/etc/os-release"
	StateFileName = "elemental-install-state.yaml"
)

func (s *ElementalInstallState) hostNeedsUpgrade(imageURI string) bool {
	return s.LastAppliedURI != imageURI
}

func LoadInstallState(fs vfs.FS, workDir string) (*ElementalInstallState, error) {
	path := formatInstallStatePath(workDir)
	bytes, err := fs.ReadFile(path)
	if os.IsNotExist(err) {
		return &ElementalInstallState{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading file '%s': %w", path, err)
	}
	var state ElementalInstallState
	if err := yaml.Unmarshal(bytes, &state); err != nil {
		return nil, fmt.Errorf("unmashalling install state file '%s': %w", path, err)
	}
	return &state, nil
}

func WriteInstallState(fs vfs.FS, workDir string, state ElementalInstallState) error {
	bytes, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshalling ElementalInstallState: %w", err)
	}
	utils.WriteFile(fs, formatInstallStatePath(workDir), bytes)
	return nil
}

func GetCurrentOSReleaseHash(fs vfs.FS) (string, error) {
	bytes, err := fs.ReadFile(OSReleasePath)
	if err != nil {
		return "", fmt.Errorf("reading file '%s': %w", OSReleasePath, err)
	}

	hash := sha256.New()
	if _, err := hash.Write(bytes); err != nil {
		//This should never be the case, but let's make the linter happy.
		return "", fmt.Errorf("writing hash of file '%s': %w", OSReleasePath, err)
	}

	result := hash.Sum(nil)
	return string(result), nil
}

func formatInstallStatePath(workDir string) string {
	return fmt.Sprintf("%s/%s", workDir, StateFileName)
}
