package utils

import (
	"errors"
	"fmt"
	"os"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/twpayne/go-vfs"
)

var ErrNotADirectory = errors.New("path exists but it's not a directory")

func CreateDirectory(fs vfs.FS, path string) error {
	fileInfo, err := fs.Stat(path)
	if os.IsNotExist(err) {
		log.Debugf("Directory '%s' does not exist. Creating now.", path)
		if err := vfs.MkdirAll(fs, path, 0700); err != nil {
			return fmt.Errorf("creating directory '%s': %w", path, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading path '%s': %w", path, err)
	}
	if !fileInfo.IsDir() {
		return ErrNotADirectory
	}
	return nil
}
