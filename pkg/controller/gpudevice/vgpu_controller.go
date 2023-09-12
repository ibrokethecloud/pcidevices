package gpudevice

import (
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
)

func (h *Handler) OnVGPUChange(_ string, gpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	if gpu == nil || gpu.DeletionTimestamp != nil || gpu.Spec.NodeName != h.nodeName {
		return gpu, nil
	}

	gpuStatus, err := gpuhelper.FetchVGPUStatus(v1beta1.MdevRoot, v1beta1.SysDevRoot, v1beta1.MdevBusClassRoot, gpu.Spec.Address)
	if err != nil {
		return gpu, fmt.Errorf("error generating vgpu %s status: %v", gpu.Name, err)
	}

	// if vGPU was configured directly on OS, reconcile state with CRD
	if !gpu.Spec.Enabled && gpuStatus.ConfiguredVGPUTypeName != "" {
		gpu.Spec.Enabled = true
		gpu.Spec.VGPUTypeName = gpuStatus.ConfiguredVGPUTypeName
		return h.vGPUClient.Update(gpu)
	}

	// perform enable disable operation //
	if !reflect.DeepEqual(gpuStatus, gpu.Status) {
		gpu.Status = *gpuStatus
		return h.vGPUClient.UpdateStatus(gpu)
	}

	return nil, nil
}

func (h *Handler) SetupVGPUDevices() error {
	vGPUDevices, err := gpuhelper.IdentifyVGPU(h.options, h.nodeName)
	if err != nil {
		return nil
	}
	return h.reconcileVGPUSetup(vGPUDevices)
}

func (h *Handler) reconcileVGPUSetup(vGPUDevices []*v1beta1.VGPUDevice) error {
	set := map[string]string{
		v1beta1.NodeKeyName: h.nodeName,
	}

	vGPUList, err := h.vGPUCache.List(labels.SelectorFromSet(set))
	if err != nil {
		return err
	}

	for _, v := range vGPUDevices {
		existingVGPU := containsVGPU(v, vGPUList)
		if existingVGPU != nil {
			if !reflect.DeepEqual(v.Spec, existingVGPU.Spec) {
				existingVGPU.Spec = v.Spec
				if _, err := h.vGPUClient.Update(existingVGPU); err != nil {
					return err
				}
			}
		} else {
			if _, err := h.vGPUClient.Create(v); err != nil {
				return err
			}
		}
	}

	for _, v := range vGPUList {
		if vGPUExists := containsVGPU(v, vGPUDevices); vGPUExists == nil {
			if err := h.vGPUClient.Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}
func containsVGPU(vgpu *v1beta1.VGPUDevice, vgpuList []*v1beta1.VGPUDevice) *v1beta1.VGPUDevice {
	for _, v := range vgpuList {
		if vgpu.Name == v.Name {
			return v
		}
	}
	return nil
}
