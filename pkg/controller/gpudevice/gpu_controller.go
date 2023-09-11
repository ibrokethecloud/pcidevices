package gpudevice

import (
	"context"
	"fmt"
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/executor"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"reflect"
)

type Handler struct {
	ctx                 context.Context
	nodeName            string
	sriovGPUCache       ctl.SRIOVGPUDeviceCache
	vGPUCache           ctl.VGPUDeviceCache
	sriovGPUClient      ctl.SRIOVGPUDeviceClient
	vGPUClient          ctl.VGPUDeviceClient
	pciDeviceClaimCache ctl.PCIDeviceClaimCache
	pciDeviceClient     ctl.PCIDeviceClient
	executor            executor.Executor
}

func NewHandler(ctx context.Context, sriovGPUController ctl.SRIOVGPUDeviceController, vGPUController ctl.VGPUDeviceController) *Handler {
	return &Handler{
		ctx:            ctx,
		sriovGPUCache:  sriovGPUController.Cache(),
		sriovGPUClient: sriovGPUController,
		vGPUCache:      vGPUController.Cache(),
		vGPUClient:     vGPUController,
		executor:       executor.NewLocalExecutor(os.Environ()),
		nodeName:       os.Getenv(v1beta1.NodeEnvVarName),
	}
}

func Register(ctx context.Context, sriovGPUController ctl.SRIOVGPUDeviceController, vGPUController ctl.VGPUDeviceController) error {
	h := NewHandler(ctx, sriovGPUController, vGPUController)
	sriovGPUController.OnChange(ctx, "on-gpu-change", h.OnGPUChange)
	vGPUController.OnChange(ctx, "on-vgpu-change", h.OnVGPUChange)
	return nil
}

func (h *Handler) OnGPUChange(_ string, gpu *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	return nil, nil
}

func (h *Handler) SetupSRIOVGPUDevices() error {
	sriovDevices, err := gpuhelper.IdentifySRIOVGPU(nil, h.nodeName)
	if err != nil {
		return err
	}

	// create missing SRIOVGPUdevices, skipping GPU's which are already passed through as PCIDevices
	for _, v := range sriovDevices {
		// if pcideviceclaim already exists for SRIOVGPU, then likely this GPU is already passed through
		// skip creation of SriovGPUDevice object until PCIDeviceClaim exists
		existingClaim, err := h.pciDeviceClaimCache.Get(v.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error looking up pcideviceclaim for sriovGPUDevice %s: %v", v.Name, err)
		}
		if existingClaim != nil {
			// pciDeviceClaim exists skipping
			logrus.Debugf("skipping creation of vGPUDevice %s as PCIDeviceClaim exists")
			continue
		}

		if err := h.createOrUpdateSRIOVGPUDevice(v); err != nil {
			return err
		}
	}
	set := map[string]string{
		v1beta1.NodeKeyName: h.nodeName,
	}

	existingGPUs, err := h.sriovGPUCache.List(labels.SelectorFromSet(set))
	if err != nil {
		return err
	}

	for _, v := range existingGPUs {
		if !containsGPUDevices(v, sriovDevices) {
			if err := h.sriovGPUClient.Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("error deleting non existant GPU device %s: %v", v.Name, err)
			}
		}
	}

	return nil
}

func (h *Handler) createOrUpdateSRIOVGPUDevice(gpu *v1beta1.SRIOVGPUDevice) error {
	existingObj, err := h.sriovGPUCache.Get(gpu.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, createErr := h.sriovGPUClient.Create(gpu)
			return createErr
		}
	}

	if !reflect.DeepEqual(existingObj.Spec, gpu.Spec) {
		existingObj.Spec = gpu.Spec
		_, err := h.sriovGPUClient.Update(existingObj)
		return err
	}
	return nil
}

func containsGPUDevices(gpu *v1beta1.SRIOVGPUDevice, gpuList []*v1beta1.SRIOVGPUDevice) bool {
	for _, v := range gpuList {
		if v.Name == gpu.Name {
			return true
		}
	}
	return false
}
