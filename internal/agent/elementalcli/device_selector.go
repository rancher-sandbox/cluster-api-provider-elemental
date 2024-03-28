package elementalcli

import (
	"fmt"

	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/block"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DeviceSelectorOpIn    DeviceSelectorOperator = "In"
	DeviceSelectorOpNotIn DeviceSelectorOperator = "NotIn"
	DeviceSelectorOpGt    DeviceSelectorOperator = "Gt"
	DeviceSelectorOpLt    DeviceSelectorOperator = "Lt"

	DeviceSelectorKeyName DeviceSelectorKey = "Name"
	DeviceSelectorKeySize DeviceSelectorKey = "Size"
)

type DeviceSelector []DeviceSelectorRequirement

type DeviceSelectorKey string
type DeviceSelectorOperator string

type DeviceSelectorRequirement struct {
	Key      DeviceSelectorKey      `json:"key" mapstructure:"key,omitempty"`
	Operator DeviceSelectorOperator `json:"operator" mapstructure:"operator,omitempty"`
	Values   []string               `json:"values,omitempty" mapstructure:"values,omitempty"`
}

type DeviceSelectorHandler interface {
	FindInstallationDevice(selector DeviceSelector) (string, error)
}

func NewDeviceSelectorHandler() (DeviceSelectorHandler, error) {
	blockInfo, err := ghw.Block(ghw.WithDisableWarnings())
	if err != nil {
		log.Error(err, "Could not probe disks.")
		return nil, fmt.Errorf("Could not probe disks")
	}

	return &deviceSelectorHandler{
		disks: blockInfo.Disks,
	}, nil
}

var _ DeviceSelectorHandler = (*deviceSelectorHandler)(nil)

type deviceSelectorHandler struct {
	disks []*block.Disk
}

func (s *deviceSelectorHandler) FindInstallationDevice(selector DeviceSelector) (string, error) {
	devices := map[string]*ghw.Disk{}

	for _, disk := range s.disks {
		devices[disk.Name] = disk
	}

	for _, disk := range s.disks {
		for _, sel := range selector {
			matches, err := matches(disk, sel)
			if err != nil {
				return "", fmt.Errorf("matching selector: %w", err)
			}

			if !matches {
				log.Debugf("%s does not match selector %s", disk.Name, sel.Key)
				delete(devices, disk.Name)
				break
			}
		}
	}

	log.Debugf("%d disks matching selector", len(devices))

	for _, dev := range devices {
		return fmt.Sprintf("/dev/%s", dev.Name), nil
	}

	return "", fmt.Errorf("no device found matching selector")
}

func matches(disk *block.Disk, req DeviceSelectorRequirement) (bool, error) {
	switch req.Operator {
	case DeviceSelectorOpIn:
		return matchesIn(disk, req)
	case DeviceSelectorOpNotIn:
		return matchesNotIn(disk, req)
	case DeviceSelectorOpLt:
		return matchesLt(disk, req)
	case DeviceSelectorOpGt:
		return matchesGt(disk, req)
	default:
		return false, fmt.Errorf("unknown operator: %s", req.Operator)
	}
}

func matchesIn(disk *block.Disk, req DeviceSelectorRequirement) (bool, error) {
	if req.Key != DeviceSelectorKeyName {
		return false, fmt.Errorf("cannot use In operator on numerical values %s", req.Key)
	}

	for _, val := range req.Values {
		if val == disk.Name || val == fmt.Sprintf("/dev/%s", disk.Name) {
			return true, nil
		}
	}

	return false, nil
}
func matchesNotIn(disk *block.Disk, req DeviceSelectorRequirement) (bool, error) {
	matches, err := matchesIn(disk, req)
	return !matches, fmt.Errorf("matching NotIn: %w", err)
}
func matchesLt(disk *block.Disk, req DeviceSelectorRequirement) (bool, error) {
	if req.Key != DeviceSelectorKeySize {
		return false, fmt.Errorf("cannot use Lt operator on string values %s", req.Key)

	}

	keySize, err := resource.ParseQuantity(req.Values[0])
	if err != nil {
		return false, fmt.Errorf("failed to parse quantity %s", req.Values[0])
	}

	diskSize := resource.NewQuantity(int64(disk.SizeBytes), resource.BinarySI)

	return diskSize.Cmp(keySize) == -1, nil
}
func matchesGt(disk *block.Disk, req DeviceSelectorRequirement) (bool, error) {
	if req.Key != DeviceSelectorKeySize {
		return false, fmt.Errorf("cannot use Gt operator on string values %s", req.Key)
	}

	keySize, err := resource.ParseQuantity(req.Values[0])
	if err != nil {
		return false, fmt.Errorf("failed to parse quantity %s", req.Values[0])
	}

	diskSize := resource.NewQuantity(int64(disk.SizeBytes), resource.BinarySI)

	return diskSize.Cmp(keySize) == 1, nil
}
