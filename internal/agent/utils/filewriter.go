package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
)

// WriteFile writes the input file into the filesystem.
//
// TODO: This is meant to be an implementation of the `write_files` cloud init instruction.
//
//	See: https://cloudinit.readthedocs.io/en/latest/reference/modules.html#write-files
//
//	All the keys should be supported (for ex. owner, permissions, encoding, etc.)
func WriteFile(fs vfs.FS, file api.BootstrapFile) error {
	log.Infof("Writing file: %s", file.Path)
	dir := filepath.Dir(file.Path)
	if _, err := fs.Stat(dir); os.IsNotExist(err) {
		log.Infof("File dir '%s' does not exist. Creating now.", dir)
		if err := vfs.MkdirAll(fs, dir, 0700); err != nil {
			return fmt.Errorf("creating file directory: %w", err)
		}
	}

	f, err := fs.Create(file.Path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	if n, err := f.WriteString(file.Content); err != nil {
		return fmt.Errorf("writing file, wrote %d bytes: %w", n, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}

	return nil
}
