package gpudevice

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	ctl "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/util/executor"
	"github.com/harvester/pcidevices/pkg/util/gpuhelper"
	"github.com/sirupsen/logrus"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvpci"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Handler struct {
	ctx                 context.Context
	nodeName            string
	sriovGPUCache       ctl.SRIOVGPUDeviceCache
	vGPUCache           ctl.VGPUDeviceCache
	sriovGPUClient      ctl.SRIOVGPUDeviceClient
	vGPUClient          ctl.VGPUDeviceClient
	pciDeviceClaimCache ctl.PCIDeviceClaimCache
	executor            executor.Executor
	options             []nvpci.Option
}

func NewHandler(ctx context.Context, sriovGPUController ctl.SRIOVGPUDeviceController, vGPUController ctl.VGPUDeviceController, pciDeviceClaim ctl.PCIDeviceClaimController, options []nvpci.Option) *Handler {
	return &Handler{
		ctx:                 ctx,
		sriovGPUCache:       sriovGPUController.Cache(),
		sriovGPUClient:      sriovGPUController,
		vGPUCache:           vGPUController.Cache(),
		vGPUClient:          vGPUController,
		pciDeviceClaimCache: pciDeviceClaim.Cache(),
		executor:            executor.NewLocalExecutor(os.Environ()),
		nodeName:            os.Getenv(v1beta1.NodeEnvVarName),
		options:             options,
	}
}

// Register setups up handlers for SRIOVGPUDevices and VGPUDevices
func Register(ctx context.Context, sriovGPUController ctl.SRIOVGPUDeviceController, vGPUController ctl.VGPUDeviceController, pciDeviceClaimController ctl.PCIDeviceClaimController) error {
	h := NewHandler(ctx, sriovGPUController, vGPUController, pciDeviceClaimController, nil)
	sriovGPUController.OnChange(ctx, "on-gpu-change", h.OnGPUChange)
	vGPUController.OnChange(ctx, "on-vgpu-change", h.OnVGPUChange)
	return nil
}

func (h *Handler) OnGPUChange(_ string, gpu *v1beta1.SRIOVGPUDevice) (*v1beta1.SRIOVGPUDevice, error) {
	return nil, nil
}

// SetupSRIVGPUDevices is called by the node controller to reconcile objects on startup and predefined intervals
func (h *Handler) SetupSRIOVGPUDevices() error {
	sriovGPUDevices, err := gpuhelper.IdentifySRIOVGPU(h.options, h.nodeName)
	if err != nil {
		return err
	}
	return h.reconcileSRIOVGPUSetup(sriovGPUDevices)
}

// reconcileSRIOVGPUSetup runs the core logic to reconcile the k8s view of node with actual state on the node
func (h *Handler) reconcileSRIOVGPUSetup(sriovGPUDevices []*v1beta1.SRIOVGPUDevice) error {
	// create missing SRIOVGPUdevices, skipping GPU's which are already passed through as PCIDevices
	for _, v := range sriovGPUDevices {
		// if pcideviceclaim already exists for SRIOVGPU, then likely this GPU is already passed through
		// skip creation of SriovGPUDevice object until PCIDeviceClaim exists
		existingClaim, err := h.pciDeviceClaimCache.Get(v.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error looking up pcideviceclaim for sriovGPUDevice %s: %v", v.Name, err)
		}
		if existingClaim != nil {
			// pciDeviceClaim exists skipping
			logrus.Debugf("skipping creation of vGPUDevice %s as PCIDeviceClaim exists", existingClaim.Name)
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
		if !containsGPUDevices(v, sriovGPUDevices) {
			if err := h.sriovGPUClient.Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("error deleting non existant GPU device %s: %v", v.Name, err)
			}
		}
	}

	return nil
}

// createOrUpdateSRIOVGPUDevice will check and create GPU if one doesnt exist. If one is found it will perform an update if needed
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

// containsGPUDevices checks if gpu exists in list of devices
func containsGPUDevices(gpu *v1beta1.SRIOVGPUDevice, gpuList []*v1beta1.SRIOVGPUDevice) bool {
	for _, v := range gpuList {
		if v.Name == gpu.Name {
			return true
		}
	}
	return false
}
