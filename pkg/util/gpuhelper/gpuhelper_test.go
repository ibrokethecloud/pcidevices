package gpuhelper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvpci"
)

func Test_IdentifySRIOVGPU(t *testing.T) {
	mockPath := os.Getenv("UMOCKDEV_DIR")
	options := []nvpci.Option{}
	if mockPath != "" {
		pcidevicePath := filepath.Join(mockPath, nvpci.PCIDevicesRoot)
		options = append(options, nvpci.WithPCIDevicesRoot(pcidevicePath))
	}
	assert := require.New(t)
	devs, err := IdentifySRIOVGPU(options, "mocknode")
	assert.NoError(err, "expected no error while querying GPU devices")
	assert.Len(devs, 1, "expected to find atleast 1 GPU from packaged snapshot")
	t.Log(*devs[0])
}
