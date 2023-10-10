package utils

import (
	"fmt"

	"github.com/twpayne/go-vfs"
)

func ReadFile(fs vfs.FS, filePath string) ([]byte, error) {
	fileContents, err := fs.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file '%s': %w", filePath, err)
	}
	return fileContents, nil
}
