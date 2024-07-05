package deviceplugins

/* This file was part of the KubeVirt project, copied to this project
 * to get around private package issues.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2024 SUSE, LLC.
 *
 */

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"
	"kubevirt.io/kubevirt/pkg/safepath"
	"kubevirt.io/kubevirt/pkg/util"
	pluginapi "kubevirt.io/kubevirt/pkg/virt-handler/device-manager/deviceplugin/v1beta1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

var (
	pathToUSBDevices = "/sys/bus/usb/devices"
)

var discoverLocalUSBDevicesFunc = discoverPluggedUSBDevices

// The actual plugin
type USBDevicePlugin struct {
	socketPath   string
	stop         <-chan struct{}
	update       chan struct{}
	done         chan struct{}
	deregistered chan struct{}
	server       *grpc.Server
	resourceName string
	devices      *PluginDevices
	logger       *log.FilteredLogger

	initialized bool
	lock        *sync.Mutex
}

type PluginDevices struct {
	ID           string
	isHealthy    bool
	DevicePath   string
	Bus          string
	DeviceNumber string
}

type USBDevice struct {
	Name         string
	Manufacturer string
	Vendor       int
	Product      int
	BCD          int
	Bus          int
	DeviceNumber int
	Serial       string
	DevicePath   string
	PCIAddress   string
}

type USBDevicePluginInterface interface {
	Start(stop <-chan struct{}) error
}

func (dev *USBDevice) GetID() string {
	return fmt.Sprintf("%04x:%04x-%02d:%02d", dev.Vendor, dev.Product, dev.Bus, dev.DeviceNumber)
}

func (pd *PluginDevices) toKubeVirtDevicePlugin() *pluginapi.Device {
	healthStr := pluginapi.Healthy
	if !pd.isHealthy {
		healthStr = pluginapi.Unhealthy
	}
	return &pluginapi.Device{
		ID:       pd.ID,
		Health:   healthStr,
		Topology: nil,
	}
}

func (plugin *USBDevicePlugin) FindDevice() *PluginDevices {
	return plugin.devices
}

func (plugin *USBDevicePlugin) setDeviceHealth(isHealthy bool) {
	pd := plugin.devices
	isDifferent := pd.isHealthy != isHealthy
	pd.isHealthy = isHealthy
	if isDifferent {
		plugin.update <- struct{}{}
	}
}

func (plugin *USBDevicePlugin) devicesToKubeVirtDevicePlugin() []*pluginapi.Device {
	devices := []*pluginapi.Device{}
	devices = append(devices, plugin.devices.toKubeVirtDevicePlugin())
	return devices
}

func (plugin *USBDevicePlugin) GetInitialized() bool {
	plugin.lock.Lock()
	defer plugin.lock.Unlock()
	return plugin.initialized
}

func (plugin *USBDevicePlugin) setInitialized(initialized bool) {
	plugin.lock.Lock()
	plugin.initialized = initialized
	plugin.lock.Unlock()
}

func (plugin *USBDevicePlugin) GetResourceName() string {
	return plugin.resourceName
}

func (plugin *USBDevicePlugin) StopDevicePlugin() error {
	defer func() {
		select {
		case <-plugin.done:
			return
		default:
			close(plugin.done)
		}
	}()

	// Give the device plugin one second to properly deregister
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	select {
	case <-plugin.deregistered:
	case <-ticker.C:
	}

	plugin.server.Stop()
	plugin.setInitialized(false)
	return plugin.cleanup()
}

func (plugin *USBDevicePlugin) Start(stop <-chan struct{}) error {
	plugin.stop = stop
	plugin.done = make(chan struct{})
	plugin.deregistered = make(chan struct{})

	err := plugin.cleanup()
	if err != nil {
		return fmt.Errorf("error on cleanup: %v", err)
	}

	sock, err := net.Listen("unix", plugin.socketPath)
	if err != nil {
		return fmt.Errorf("error creating GRPC server socket: %v", err)
	}

	plugin.server = grpc.NewServer([]grpc.ServerOption{}...)
	defer plugin.StopDevicePlugin()

	pluginapi.RegisterDevicePluginServer(plugin.server, plugin)

	errChan := make(chan error, 2)

	go func() {
		errChan <- plugin.server.Serve(sock)
	}()

	err = waitForGRPCServer(context.Background(), plugin.socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("error starting the GRPC server: %v", err)
	}

	err = plugin.register()
	if err != nil {
		return fmt.Errorf("error registering with device plugin manager: %v", err)
	}

	go func() {
		errChan <- plugin.healthCheck()
	}()

	plugin.setInitialized(true)
	plugin.logger.Infof("%s device plugin started", plugin.resourceName)
	err = <-errChan

	return err
}

func (plugin *USBDevicePlugin) healthCheck() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to creating a fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	monitoredDevices, err := plugin.getMonitoredDevices(watcher)
	if err != nil {
		logrus.Errorf("failed to get monitored devices: %v", err)
		return err
	}

	err = plugin.addSocketPath(watcher)
	if err != nil {
		logrus.Errorf("failed to add socket path to watcher: %v", err)
		return err
	}

	for {
		select {
		case <-plugin.stop:
			return nil
		case err := <-watcher.Errors:
			plugin.logger.Reason(err).Errorf("error watching devices and device plugin directory")
		case event := <-watcher.Events:
			plugin.logger.V(2).Infof("health Event: %v", event)
			if _, exist := monitoredDevices[event.Name]; exist {
				// Health in this case is if the device path actually exists
				if event.Op == fsnotify.Create {
					plugin.logger.Infof("monitored device %s appeared", plugin.resourceName)
					plugin.setDeviceHealth(true)
				} else if (event.Op == fsnotify.Remove) || (event.Op == fsnotify.Rename) {
					plugin.logger.Infof("monitored device %s disappeared", plugin.resourceName)
					plugin.setDeviceHealth(false)
				}
			} else if event.Name == plugin.socketPath && event.Op == fsnotify.Remove {
				plugin.logger.Infof("device socket file for device %s was removed, kubelet probably restarted.", plugin.resourceName)
				return nil
			}
		}
	}
}

func (plugin *USBDevicePlugin) addSocketPath(watcher *fsnotify.Watcher) error {
	dirName := filepath.Dir(plugin.socketPath)

	if err := watcher.Add(dirName); err != nil {
		return fmt.Errorf("failed to add the device-plugin kubelet path to the watcher: %v", err)
	} else if _, err = os.Stat(plugin.socketPath); err != nil {
		return fmt.Errorf("failed to stat the device-plugin socket: %v", err)
	}

	return nil
}

func (plugin *USBDevicePlugin) getMonitoredDevices(watcher *fsnotify.Watcher) (map[string]string, error) {
	monitoredDevices := make(map[string]string)
	watchedDirs := make(map[string]struct{})

	usbDevicePath := filepath.Join(util.HostRootMount, plugin.devices.DevicePath)
	usbDeviceDirPath := filepath.Dir(usbDevicePath)
	if _, exists := watchedDirs[usbDeviceDirPath]; !exists {
		if err := watcher.Add(usbDeviceDirPath); err != nil {
			return nil, fmt.Errorf("failed to watch device %s parent directory: %s", usbDevicePath, err)
		}
		watchedDirs[usbDeviceDirPath] = struct{}{}
	}

	if err := watcher.Add(usbDevicePath); err != nil {
		return nil, fmt.Errorf("failed to add the device %s to the watcher: %s", usbDevicePath, err)
	} else if _, err := os.Stat(usbDevicePath); err != nil {
		return nil, fmt.Errorf("failed to validate device %s: %s", usbDevicePath, err)
	}
	monitoredDevices[usbDevicePath] = plugin.devices.ID
	return monitoredDevices, nil
}

func (plugin *USBDevicePlugin) cleanup() error {
	err := os.Remove(plugin.socketPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (plugin *USBDevicePlugin) register() error {
	conn, err := grpc.Dial(pluginapi.KubeletSocket,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(plugin.socketPath),
		ResourceName: plugin.GetResourceName(),
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

func (plugin *USBDevicePlugin) GetDevicePluginOptions(_ context.Context, _ *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}, nil
}

// Interface to expose Devices: IDs, health and Topology
func (plugin *USBDevicePlugin) ListAndWatch(_ *pluginapi.Empty, lws pluginapi.DevicePlugin_ListAndWatchServer) error {
	sendUpdate := func(devices []*pluginapi.Device) error {
		response := pluginapi.ListAndWatchResponse{
			Devices: devices,
		}
		err := lws.Send(&response)
		if err != nil {
			plugin.logger.Reason(err).Warningf("Failed to send device plugin %s",
				plugin.resourceName)
		}
		return err
	}

	if err := sendUpdate(plugin.devicesToKubeVirtDevicePlugin()); err != nil {
		return err
	}
	done := false
	for !done {
		select {
		case <-plugin.update:
			if err := sendUpdate(plugin.devicesToKubeVirtDevicePlugin()); err != nil {
				return err
			}
		case <-plugin.stop:
			done = true
		}
	}

	if err := sendUpdate([]*pluginapi.Device{}); err != nil {
		plugin.logger.Reason(err).Warningf("Failed to deregister device plugin %s",
			plugin.resourceName)
	}
	close(plugin.deregistered)
	return nil
}

// Interface to allocate requested USBDevice, exported by ListAndWatch
func (plugin *USBDevicePlugin) Allocate(_ context.Context, allocRequest *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	allocResponse := new(pluginapi.AllocateResponse)
	env := make(map[string]string)
	for _, request := range allocRequest.ContainerRequests {
		containerResponse := &pluginapi.ContainerAllocateResponse{}
		for _, id := range request.DevicesIDs {
			plugin.logger.V(2).Infof("usb device id: %s", id)

			pluginDevice := plugin.FindDevice()
			if pluginDevice == nil {
				plugin.logger.V(2).Infof("usb disappeared: %s", id)
				continue
			}

			deviceSpecs := []*pluginapi.DeviceSpec{}
			spath, err := safepath.JoinAndResolveWithRelativeRoot(util.HostRootMount, pluginDevice.DevicePath)
			if err != nil {
				return nil, fmt.Errorf("error opening the socket %s: %v", pluginDevice.DevicePath, err)
			}

			err = safepath.ChownAtNoFollow(spath, util.NonRootUID, util.NonRootUID)
			if err != nil {
				return nil, fmt.Errorf("error setting the permission the socket %s: %v", pluginDevice.DevicePath, err)
			}

			key := util.ResourceNameToEnvVar(v1.USBResourcePrefix, plugin.resourceName)
			value := fmt.Sprintf("%d:%d", pluginDevice.Bus, pluginDevice.DeviceNumber)

			deviceSpecs = append(deviceSpecs, &pluginapi.DeviceSpec{
				ContainerPath: pluginDevice.DevicePath,
				HostPath:      pluginDevice.DevicePath,
				Permissions:   "mrw",
			})

			if previous, exist := env[key]; exist {
				env[key] = fmt.Sprintf("%s,%s", previous, value)
			} else {
				env[key] = value
			}
			containerResponse.Envs = env
			containerResponse.Devices = append(containerResponse.Devices, deviceSpecs...)
		}
		allocResponse.ContainerResponses = append(allocResponse.ContainerResponses, containerResponse)
	}

	for _, v := range allocResponse.ContainerResponses {
		logrus.Infof("%v\n", *v)
	}
	return allocResponse, nil
}

func (plugin *USBDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func NewUSBDevicePlugin(usb v1beta1.USBDevice) *USBDevicePlugin {
	s := strings.Split(usb.Spec.ResourceName, "/")
	resourceID := s[0]
	if len(s) > 1 {
		resourceID = s[1]
	}
	bus, deviceNumber := generateBusAndDevice(usb.Spec.DevicePath)
	resourceID = fmt.Sprintf("usb-%s", resourceID)
	return &USBDevicePlugin{
		socketPath:   SocketPath(resourceID),
		resourceName: usb.Spec.ResourceName,
		devices: &PluginDevices{
			ID:           usb.Spec.DevicePath,
			isHealthy:    true,
			Bus:          bus,
			DeviceNumber: deviceNumber,
			DevicePath:   usb.Spec.DevicePath,
		},
		logger:      log.Log.With("subcomponent", resourceID),
		initialized: false,
		lock:        &sync.Mutex{},
	}
}

func (plugin *USBDevicePlugin) StartDevicePlugin() {
	if plugin.initialized {
		return
	}

	stop := make(chan struct{})

	go func() {
		for {
			// This will be blocked by a channel read inside function
			if err := plugin.Start(stop); err != nil {
				logrus.Errorf("Error starting %s device plugin", plugin.resourceName)
			}

			select {
			case <-stop:
				return
			case <-time.After(5 * time.Second):
				// try to start device plugin again when getting error
				continue
			}
		}
	}()

	plugin.initialized = true
}

func generateBusAndDevice(devicePath string) (string, string) {
	result := strings.Split(devicePath, "/")
	return result[len(result)-2], result[len(result)-1]
}
