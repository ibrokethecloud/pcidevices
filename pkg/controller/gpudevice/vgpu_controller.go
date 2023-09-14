package gpudevice

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
)

var (
	pluginLock sync.Mutex
)

func (h *Handler) OnVGPUChange(_ string, vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	if vgpu == nil || vgpu.DeletionTimestamp != nil || vgpu.Spec.NodeName != h.nodeName {
		return vgpu, nil
	}

	discoveredVGPUStatus, err := gpuhelper.FetchVGPUStatus(v1beta1.MdevRoot, v1beta1.SysDevRoot, v1beta1.MdevBusClassRoot, vgpu.Spec.Address)
	if err != nil {
		return vgpu, fmt.Errorf("error generating vgpu %s status: %v", vgpu.Name, err)
	}

	// gpu spec is enabled and discovered status indicates no configuration
	if vgpu.Spec.Enabled && discoveredVGPUStatus.VGPUStatus == v1beta1.VGPUDisabled {
		return h.enableVGPU(vgpu)
	}

	if !vgpu.Spec.Enabled && discoveredVGPUStatus.VGPUStatus == v1beta1.VGPUEnabled {
		return h.disableVGPU(vgpu)
	}
	// perform enable disable operation //
	if !reflect.DeepEqual(discoveredVGPUStatus, vgpu.Status) {
		vgpu.Status = *discoveredVGPUStatus
		return h.vGPUClient.UpdateStatus(vgpu)
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

// enableVGPU performs the op to configure VGPU
func (h *Handler) enableVGPU(vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	nvidiaType, ok := vgpu.Status.AvailableTypes[vgpu.Spec.VGPUTypeName]
	if !ok {
		return vgpu, fmt.Errorf("VGPUType specified %s is not available for vGPU %s", vgpu.Spec.VGPUTypeName, vgpu.Spec.Address)
	}

	vgpuUUID := uuid.NewString()

	createFilePath := filepath.Join(v1beta1.MdevBusClassRoot, vgpu.Spec.Address, v1beta1.MdevSupportTypesDir, nvidiaType, "create")
	if _, err := os.Stat(createFilePath); err != nil {
		return vgpu, fmt.Errorf("error looking up create file for vgpu %s: %v", vgpu.Name, err)
	}

	if err := os.WriteFile(createFilePath, []byte(vgpuUUID), fs.FileMode(os.O_WRONLY)); err != nil {
		return vgpu, fmt.Errorf("error writing to create file for vgpu %s: %v", vgpu.Name, err)
	}

	vgpu.Status.VGPUStatus = v1beta1.VGPUEnabled
	vgpu.Status.UUID = vgpuUUID
	return h.vGPUClient.UpdateStatus(vgpu)
}

// disableVGPU performs the op to disable VGPU
func (h *Handler) disableVGPU(vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	removeFile := filepath.Join(v1beta1.MdevBusClassRoot, vgpu.Spec.Address, vgpu.Status.UUID, "remove")
	var notFound bool
	// possible that CRD update fails but file has been removed
	// this can lead to issue during reconcile.
	// in such a case we just ensure plugin is updated and CRD status reflects disabled state
	if _, err := os.Stat(removeFile); err != nil {
		if os.IsNotExist(err) {
			notFound = true
		} else {
			return vgpu, fmt.Errorf("error looking up remove file for vgpu %s: %v", vgpu.Name, err)
		}
	}

	if !notFound {
		if err := os.WriteFile(removeFile, []byte("1"), fs.FileMode(os.O_WRONLY)); err != nil {
			return vgpu, fmt.Errorf("error writing to remove file for vgpu %s: %v", vgpu.Name, err)
		}
	}

	// disableDevicePlugin is run here as we need UUID to remove the device
	if err := h.disableDevicePlugin(vgpu); err != nil {
		return vgpu, fmt.Errorf("error cleaning up device plugin for device %s: %v", vgpu.Name, err)
	}

	vgpu.Status.VGPUStatus = v1beta1.VGPUDisabled
	vgpu.Status.UUID = ""
	vgpu.Status.ConfiguredVGPUTypeName = ""
	return h.vGPUClient.UpdateStatus(vgpu)
}

func (h *Handler) patchKubevirtCR() error {
	return nil
}

func (h *Handler) disableDevicePlugin(vgpu *v1beta1.VGPUDevice) error {
	pluginLock.Lock()
	defer pluginLock.Unlock()
	pluginName := gpuhelper.GenerateDeviceName(vgpu.Status.ConfiguredVGPUTypeName)
	plugin, ok := h.vGPUDevicePlugins[pluginName]
	if !ok {
		logrus.Debugf("no device plugin found for vgpu %s of type %s", vgpu.Name, vgpu.Status.ConfiguredVGPUTypeName)
		return nil
	}

	plugin.RemoveDevice(vgpu.Status.UUID)
	if plugin.Count() == 0 {
		logrus.Infof("shutting down device plugin for %s", pluginName)
		return plugin.Stop()
	}
	return nil
}

// reconcileEnabledVGPUPlugins runs as an out of band handler from the VGPU Device management loop. This is needed as we reconcile CRD to OS state.
// in case there was an error during CRD status update, subsequent reconcile will generate correct status from CRD.
// the enable subroutine is skipped in this case and placing the device plugin enable logic will likely miss some devices
func (h *Handler) reconcileEnabledVGPUPlugins(_ string, vgpu *v1beta1.VGPUDevice) (*v1beta1.VGPUDevice, error) {
	if vgpu == nil || vgpu.DeletionTimestamp != nil || vgpu.Spec.NodeName != h.nodeName {
		return vgpu, nil
	}

	if vgpu.Spec.Enabled && vgpu.Status.UUID != "" {
		return vgpu, h.createOrUpdateDevicePlugin(vgpu)
	}

	return vgpu, nil
}

func (h *Handler) createOrUpdateDevicePlugin(vgpu *v1beta1.VGPUDevice) error {
	pluginLock.Lock()
	defer pluginLock.Unlock()
	pluginName := gpuhelper.GenerateDeviceName(vgpu.Status.ConfiguredVGPUTypeName)
	plugin, ok := h.vGPUDevicePlugins[pluginName]
	if ok {
		// plugin exists. just publish address and move on
		if !plugin.DeviceExists(vgpu.Status.UUID) {
			plugin.AddDevice(vgpu.Status.UUID)
		}
		return nil
	}

	newPlugin := deviceplugins.NewVGPUDevicePlugin(h.ctx, []string{vgpu.Status.UUID}, pluginName)
	if err := h.startDevicePlugin(newPlugin); err != nil {
		return err
	}

	h.vGPUDevicePlugins[pluginName] = newPlugin
	return nil
}

func (h *Handler) startDevicePlugin(
	dp *deviceplugins.VGPUDevicePlugin,
) error {
	if dp.Started() {
		return nil
	}
	// Start the plugin
	stop := make(chan struct{})
	go func() {
		err := dp.Start(stop)
		if err != nil {
			logrus.Errorf("error starting %s device plugin: %s", dp.GetDeviceName(), err)
		}
		// TODO: test if deleting this stops the DevicePlugin
		<-stop
	}()
	dp.SetStarted(stop)
	return nil
}
