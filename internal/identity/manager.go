package identity

import (
	"fmt"
	"os"

	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
)

const (
	PrivateKeyFile = "private.key"
)

type Manager interface {
	LoadSigningKeyOrCreateNew() (Identity, error)
}

var _ Manager = (*manager)(nil)

type manager struct {
	workDir string
	fs      vfs.FS
}

func NewManager(fs vfs.FS, workDir string) Manager {
	return &manager{
		workDir: workDir,
		fs:      fs,
	}
}

func (m *manager) LoadSigningKeyOrCreateNew() (Identity, error) {
	identity := &Ed25519Identity{}

	path := fmt.Sprintf("%s/%s", m.workDir, PrivateKeyFile)
	log.Debugf("Loading identity from file: %s", path)
	_, err := m.fs.Stat(path)
	if os.IsNotExist(err) {
		log.Debug("Identity file does not exist, creating a new one")
		identity, err := NewED25519Identity()
		if err != nil {
			return nil, fmt.Errorf("creating new Ed25519 identity: %w", err)
		}
		return identity, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting '%s' file info: %w", path, err)
	}
	key, err := m.fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading '%s': %w", path, err)
	}
	if err := identity.Unmarshal(key); err != nil {
		return nil, fmt.Errorf("unmarshalling private key: %w", err)
	}
	return identity, nil
}
