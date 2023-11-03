package identity

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/twpayne/go-vfs"
)

const (
	PrivateKeyFile = "private.key"
)

type Manager interface {
	LoadSigningKeyOrCreateNew() ([]byte, error)
}

var _ Manager = (*DummyManager)(nil)

type DummyManager struct {
	workDir string
	fs      vfs.FS
}

func NewDummyManager(fs vfs.FS, workDir string) Manager {
	return &DummyManager{
		workDir: workDir,
		fs:      fs,
	}
}

func (m *DummyManager) LoadSigningKeyOrCreateNew() ([]byte, error) {
	path := fmt.Sprintf("%s/%s", m.workDir, PrivateKeyFile)
	log.Debugf("Loading dummy key: %s", path)
	_, err := m.fs.Stat(path)
	if os.IsNotExist(err) {
		log.Debug("Dummy key does not exist, creating a new one")
		key, err := m.generateNewKey()
		if err != nil {
			return nil, fmt.Errorf("generating new key: %w", err)
		}
		return key, nil
	}
	key, err := utils.ReadFile(m.fs, path)
	if err != nil {
		return nil, fmt.Errorf("loading '%s': %w", path, err)
	}
	return key, nil
}

func (m *DummyManager) generateNewKey() ([]byte, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generating new random UUID: %w", err)
	}
	return []byte(uuid.String()), nil
}
