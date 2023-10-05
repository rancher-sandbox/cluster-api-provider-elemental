package host

import (
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
)

const (
	IdentityDirPathDefault = "/var/lib/elemental"
	IdentityFile           = "private.key"
)

var ErrIdentityDoesNotExist = errors.New("no identity found")

type IdentityManager interface {
	GetOrCreateIdentity() (Identity, error)
}

var _ IdentityManager = (*DummyManager)(nil)

type DummyManager struct {
	dirPath string
	fs      vfs.FS
}

func NewDummyManager(fs vfs.FS, dirPath string) IdentityManager {
	return &DummyManager{
		dirPath: dirPath,
		fs:      fs,
	}
}

func (m *DummyManager) GetOrCreateIdentity() (Identity, error) {
	log.Debugf("Getting dummy identity in dir: %s", m.dirPath)
	identity := &DummyIdentity{}
	err := identity.LoadFromFile(m.fs, m.formatFilePath())

	if errors.Is(err, ErrIdentityDoesNotExist) {
		log.Debug("Identity does not exist yet")
		identity, err := m.newIdentity()
		if err != nil {
			return nil, fmt.Errorf("creating new identity: %w", err)
		}
		return identity, nil
	}

	if err != nil {
		return nil, fmt.Errorf("loading dummy identity: %w", err)
	}
	return identity, nil
}

func (m *DummyManager) newIdentity() (*DummyIdentity, error) {
	log.Debug("Creating new dummy key")
	identity, err := NewDummyIdentity()
	if err != nil {
		return nil, fmt.Errorf("initializing new dummy identity: %w", err)
	}
	if err := identity.WriteToFile(m.fs, m.formatFilePath()); err != nil {
		return nil, fmt.Errorf("writing dummy identity to file: %w", err)
	}
	return identity, nil
}

func (m *DummyManager) formatFilePath() string {
	return fmt.Sprintf("%s/%s", IdentityDirPathDefault, IdentityFile)
}

type Identity interface {
	GetSigningKey() ([]byte, error)
}

var _ Identity = (*DummyIdentity)(nil)

type DummyIdentity struct {
	key []byte
}

func NewDummyIdentity() (*DummyIdentity, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generating new random UUID: %w", err)
	}
	return &DummyIdentity{
		key: []byte(uuid.String()),
	}, nil
}

func (i *DummyIdentity) GetSigningKey() ([]byte, error) {
	return nil, nil
}

func (i *DummyIdentity) LoadFromFile(fs vfs.FS, filePath string) error {
	log.Debugf("Loading dummy identity from file: %s", filePath)
	key, err := utils.ReadFile(fs, filePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("loading '%s': %w", filePath, ErrIdentityDoesNotExist)
	}
	if err != nil {
		return fmt.Errorf("loading '%s': %w", filePath, err)
	}
	i.key = key
	return nil
}

func (i *DummyIdentity) WriteToFile(fs vfs.FS, filePath string) error {
	log.Debugf("Writing dummy identity to file: %s", filePath)
	if err := utils.WriteFile(fs, api.WriteFile{
		Path:    filePath,
		Content: string(i.key),
	}); err != nil {
		return fmt.Errorf("writing key to file '%s': %w", filePath, err)
	}
	return nil
}
