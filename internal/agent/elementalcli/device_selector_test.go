package elementalcli

import (
	"github.com/jaypipes/ghw"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Device Selector", Label("agent", "plugin", "elemental", "device-selector"), func() {
	var deviceSelector DeviceSelectorHandler
	It("should pick single device no selectors", func() {
		deviceSelector = &deviceSelectorHandler{
			disks: []*ghw.Disk{{Name: "pickme"}},
		}
		actualDevice, err := deviceSelector.FindInstallationDevice(DeviceSelector{})
		Expect(err).ToNot(HaveOccurred())
		Expect(actualDevice).To(Equal("/dev/pickme"))
	})
	It("should pick device based on selector name", func() {
		deviceSelector = &deviceSelectorHandler{
			disks: []*ghw.Disk{
				{Name: "sda"},
				{Name: "sdb"},
				{Name: "sdc"},
				{Name: "sdd"},
				{Name: "sde"},
				{Name: "sdf"},
				{Name: "sdg"},
			},
		}
		selector := DeviceSelector{
			{
				Key:      DeviceSelectorKeyName,
				Operator: DeviceSelectorOpIn,
				Values:   []string{"/dev/sdd"},
			},
		}

		actualDevice, err := deviceSelector.FindInstallationDevice(selector)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualDevice).To(Equal("/dev/sdd"))
	})
	It("should pick device less than 100Gi", func() {
		deviceSelector = &deviceSelectorHandler{
			disks: []*ghw.Disk{
				{Name: "sda", SizeBytes: 85899345920},
				{Name: "sdb", SizeBytes: 214748364800},
			},
		}
		selector := DeviceSelector{
			{
				Key:      DeviceSelectorKeySize,
				Operator: DeviceSelectorOpLt,
				Values:   []string{"100Gi"},
			},
		}

		actualDevice, err := deviceSelector.FindInstallationDevice(selector)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualDevice).To(Equal("/dev/sda"))
	})
	It("should pick device greater than 100Gi", func() {
		deviceSelector = &deviceSelectorHandler{
			disks: []*ghw.Disk{
				{Name: "sda", SizeBytes: 85899345920},
				{Name: "sdb", SizeBytes: 214748364800},
			},
		}
		selector := DeviceSelector{
			{
				Key:      DeviceSelectorKeySize,
				Operator: DeviceSelectorOpGt,
				Values:   []string{"100Gi"},
			},
		}

		actualDevice, err := deviceSelector.FindInstallationDevice(selector)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualDevice).To(Equal("/dev/sdb"))
	})
	It("should not error out for 2 matching devices", func() {
		deviceSelector = &deviceSelectorHandler{
			disks: []*ghw.Disk{
				{Name: "sda"},
				{Name: "sdb"},
			},
		}
		selector := DeviceSelector{
			{
				Key:      DeviceSelectorKeyName,
				Operator: DeviceSelectorOpIn,
				Values:   []string{"/dev/sda", "/dev/sdb"},
			},
		}
		actualDevice, err := deviceSelector.FindInstallationDevice(selector)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualDevice).ToNot(BeEmpty())
	})
	It("should error out for no devices", func() {
		deviceSelector = &deviceSelectorHandler{
			disks: []*ghw.Disk{},
		}
		actualDevice, err := deviceSelector.FindInstallationDevice(DeviceSelector{})
		Expect(err).To(HaveOccurred())
		Expect(actualDevice).To(BeEmpty())
	})
})
