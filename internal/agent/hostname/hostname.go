package hostname

import (
	"fmt"
	"os"

	"github.com/google/uuid"
)

func FormatRandom(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, uuid.New().String())
}

func FormatCurrent(prefix string) (string, error) {
	currentHostname, err := GetCurrent()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return fmt.Sprintf("%s%s", prefix, currentHostname), nil
}

func GetCurrent() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return hostname, nil
}
