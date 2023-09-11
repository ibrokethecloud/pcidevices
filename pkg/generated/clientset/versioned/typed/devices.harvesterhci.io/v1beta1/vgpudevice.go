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
	"context"
	"time"

	v1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	scheme "github.com/harvester/pcidevices/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// VGPUDevicesGetter has a method to return a VGPUDeviceInterface.
// A group's client should implement this interface.
type VGPUDevicesGetter interface {
	VGPUDevices() VGPUDeviceInterface
}

// VGPUDeviceInterface has methods to work with VGPUDevice resources.
type VGPUDeviceInterface interface {
	Create(ctx context.Context, vGPUDevice *v1beta1.VGPUDevice, opts v1.CreateOptions) (*v1beta1.VGPUDevice, error)
	Update(ctx context.Context, vGPUDevice *v1beta1.VGPUDevice, opts v1.UpdateOptions) (*v1beta1.VGPUDevice, error)
	UpdateStatus(ctx context.Context, vGPUDevice *v1beta1.VGPUDevice, opts v1.UpdateOptions) (*v1beta1.VGPUDevice, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.VGPUDevice, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.VGPUDeviceList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.VGPUDevice, err error)
	VGPUDeviceExpansion
}

// vGPUDevices implements VGPUDeviceInterface
type vGPUDevices struct {
	client rest.Interface
}

// newVGPUDevices returns a VGPUDevices
func newVGPUDevices(c *DevicesV1beta1Client) *vGPUDevices {
	return &vGPUDevices{
		client: c.RESTClient(),
	}
}

// Get takes name of the vGPUDevice, and returns the corresponding vGPUDevice object, and an error if there is any.
func (c *vGPUDevices) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.VGPUDevice, err error) {
	result = &v1beta1.VGPUDevice{}
	err = c.client.Get().
		Resource("vgpudevices").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of VGPUDevices that match those selectors.
func (c *vGPUDevices) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.VGPUDeviceList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.VGPUDeviceList{}
	err = c.client.Get().
		Resource("vgpudevices").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested vGPUDevices.
func (c *vGPUDevices) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("vgpudevices").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a vGPUDevice and creates it.  Returns the server's representation of the vGPUDevice, and an error, if there is any.
func (c *vGPUDevices) Create(ctx context.Context, vGPUDevice *v1beta1.VGPUDevice, opts v1.CreateOptions) (result *v1beta1.VGPUDevice, err error) {
	result = &v1beta1.VGPUDevice{}
	err = c.client.Post().
		Resource("vgpudevices").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(vGPUDevice).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a vGPUDevice and updates it. Returns the server's representation of the vGPUDevice, and an error, if there is any.
func (c *vGPUDevices) Update(ctx context.Context, vGPUDevice *v1beta1.VGPUDevice, opts v1.UpdateOptions) (result *v1beta1.VGPUDevice, err error) {
	result = &v1beta1.VGPUDevice{}
	err = c.client.Put().
		Resource("vgpudevices").
		Name(vGPUDevice.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(vGPUDevice).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *vGPUDevices) UpdateStatus(ctx context.Context, vGPUDevice *v1beta1.VGPUDevice, opts v1.UpdateOptions) (result *v1beta1.VGPUDevice, err error) {
	result = &v1beta1.VGPUDevice{}
	err = c.client.Put().
		Resource("vgpudevices").
		Name(vGPUDevice.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(vGPUDevice).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the vGPUDevice and deletes it. Returns an error if one occurs.
func (c *vGPUDevices) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("vgpudevices").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *vGPUDevices) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("vgpudevices").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched vGPUDevice.
func (c *vGPUDevices) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.VGPUDevice, err error) {
	result = &v1beta1.VGPUDevice{}
	err = c.client.Patch(pt).
		Resource("vgpudevices").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
