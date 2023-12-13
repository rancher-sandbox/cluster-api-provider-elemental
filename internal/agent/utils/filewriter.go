package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/twpayne/go-vfs"
)

func WriteFile(fs vfs.FS, path string, content []byte) error {
	log.Infof("Writing file: %s", path)
	dir := filepath.Dir(path)
	if _, err := fs.Stat(dir); os.IsNotExist(err) {
		log.Infof("File dir '%s' does not exist. Creating now.", dir)
		if err := vfs.MkdirAll(fs, dir, 0700); err != nil {
			return fmt.Errorf("creating file directory: %w", err)
		}
	}

	f, err := fs.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	if n, err := f.Write(content); err != nil {
		return fmt.Errorf("writing file, wrote %d bytes: %w", n, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}

	return nil
}
