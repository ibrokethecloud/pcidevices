/*
Copyright 2022 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v1beta1

import (
	"net/http"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type DevicesV1beta1Interface interface {
	RESTClient() rest.Interface
	NodesGetter
	PCIDevicesGetter
	PCIDeviceClaimsGetter
	SRIOVGPUDevicesGetter
	SRIOVNetworkDevicesGetter
	USBDevicesGetter
	USBDeviceClaimsGetter
	VGPUDevicesGetter
}

// DevicesV1beta1Client is used to interact with features provided by the devices.harvesterhci.io group.
type DevicesV1beta1Client struct {
	restClient rest.Interface
}

func (c *DevicesV1beta1Client) Nodes() NodeInterface {
	return newNodes(c)
}

func (c *DevicesV1beta1Client) PCIDevices() PCIDeviceInterface {
	return newPCIDevices(c)
}

func (c *DevicesV1beta1Client) PCIDeviceClaims() PCIDeviceClaimInterface {
	return newPCIDeviceClaims(c)
}

func (c *DevicesV1beta1Client) SRIOVGPUDevices() SRIOVGPUDeviceInterface {
	return newSRIOVGPUDevices(c)
}

func (c *DevicesV1beta1Client) SRIOVNetworkDevices() SRIOVNetworkDeviceInterface {
	return newSRIOVNetworkDevices(c)
}

func (c *DevicesV1beta1Client) USBDevices() USBDeviceInterface {
	return newUSBDevices(c)
}

func (c *DevicesV1beta1Client) USBDeviceClaims() USBDeviceClaimInterface {
	return newUSBDeviceClaims(c)
}

func (c *DevicesV1beta1Client) VGPUDevices() VGPUDeviceInterface {
	return newVGPUDevices(c)
}

// NewForConfig creates a new DevicesV1beta1Client for the given config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*DevicesV1beta1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(&config, httpClient)
}

// NewForConfigAndClient creates a new DevicesV1beta1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*DevicesV1beta1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &DevicesV1beta1Client{client}, nil
}

// NewForConfigOrDie creates a new DevicesV1beta1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *DevicesV1beta1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new DevicesV1beta1Client for the given RESTClient.
func New(c rest.Interface) *DevicesV1beta1Client {
	return &DevicesV1beta1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1beta1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *DevicesV1beta1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
