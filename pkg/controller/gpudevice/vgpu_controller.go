package gpudevice

import "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"

func (h *Handler) OnVGPUChange(_ string, gpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	return nil, nil
}
