package gpuhelper

import (
	"fmt"
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/common"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvpci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IdentifySRIOVGPU(options []nvpci.Option, hostname string) ([]*v1beta1.SRIOVGPUDevice, error) {
	mgr := nvpci.New(options...)
	sriovGPUDevices := make([]*v1beta1.SRIOVGPUDevice, 0)
	nvidiaGPU, err := mgr.GetGPUs()
	if err != nil {
		return nil, fmt.Errorf("error querying GPU's: %v", err)
	}

	for _, v := range nvidiaGPU {
		ok, err := common.IsDeviceSRIOVCapable(v.Path)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		devObj := generateSRIOVGPUDevice(v, hostname)
		enabled, devObjStatus, err := generateGPUStatus(v.Path, hostname)
		if err != nil {
			return nil, err
		}
		devObj.Spec.Enabled = enabled
		devObj.Status = *devObjStatus
		sriovGPUDevices = append(sriovGPUDevices, generateSRIOVGPUDevice(v, hostname))

	}

	return sriovGPUDevices, nil
}

func generateSRIOVGPUDevice(nvidiaGpu *nvpci.NvidiaPCIDevice, hostname string) *v1beta1.SRIOVGPUDevice {
	obj := &v1beta1.SRIOVGPUDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.PCIDeviceNameForHostname(nvidiaGpu.Address, hostname),
		},
		Spec: v1beta1.SRIOVGPUDeviceSpec{
			Address:  nvidiaGpu.Address,
			NodeName: hostname,
		},
	}
	return obj
}

func generateGPUStatus(devicePath string, hostname string) (bool, *v1beta1.SRIOVGPUDeviceStatus, error) {
	var enabled bool
	var err error
	logrus.Infof("checking devicePath: %s", devicePath)
	count, err := common.CurrentVFConfigured(devicePath)
	if err != nil {
		return enabled, nil, err
	}

	if count > 0 {
		enabled = true
	}

	vfs, err := common.GetVFList(devicePath)
	if err != nil {
		return enabled, nil, err
	}
	logrus.Info("found vfs %s", vfs)

	deviceStatus := &v1beta1.SRIOVGPUDeviceStatus{}
	deviceStatus.VFAddresses = vfs
	for _, v := range vfs {
		deviceStatus.VGPUDevices = append(deviceStatus.VGPUDevices, v1beta1.PCIDeviceNameForHostname(v, hostname))
	}
	return enabled, deviceStatus, nil
}
