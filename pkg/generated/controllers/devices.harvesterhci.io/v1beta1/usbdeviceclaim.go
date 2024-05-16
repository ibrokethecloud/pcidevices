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
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kv"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type USBDeviceClaimHandler func(string, *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error)

type USBDeviceClaimController interface {
	generic.ControllerMeta
	USBDeviceClaimClient

	OnChange(ctx context.Context, name string, sync USBDeviceClaimHandler)
	OnRemove(ctx context.Context, name string, sync USBDeviceClaimHandler)
	Enqueue(name string)
	EnqueueAfter(name string, duration time.Duration)

	Cache() USBDeviceClaimCache
}

type USBDeviceClaimClient interface {
	Create(*v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error)
	Update(*v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error)
	UpdateStatus(*v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error)
	Delete(name string, options *metav1.DeleteOptions) error
	Get(name string, options metav1.GetOptions) (*v1beta1.USBDeviceClaim, error)
	List(opts metav1.ListOptions) (*v1beta1.USBDeviceClaimList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.USBDeviceClaim, err error)
}

type USBDeviceClaimCache interface {
	Get(name string) (*v1beta1.USBDeviceClaim, error)
	List(selector labels.Selector) ([]*v1beta1.USBDeviceClaim, error)

	AddIndexer(indexName string, indexer USBDeviceClaimIndexer)
	GetByIndex(indexName, key string) ([]*v1beta1.USBDeviceClaim, error)
}

type USBDeviceClaimIndexer func(obj *v1beta1.USBDeviceClaim) ([]string, error)

type uSBDeviceClaimController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewUSBDeviceClaimController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) USBDeviceClaimController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &uSBDeviceClaimController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromUSBDeviceClaimHandlerToHandler(sync USBDeviceClaimHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v1beta1.USBDeviceClaim
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v1beta1.USBDeviceClaim))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *uSBDeviceClaimController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v1beta1.USBDeviceClaim))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateUSBDeviceClaimDeepCopyOnChange(client USBDeviceClaimClient, obj *v1beta1.USBDeviceClaim, handler func(obj *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error)) (*v1beta1.USBDeviceClaim, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *uSBDeviceClaimController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *uSBDeviceClaimController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *uSBDeviceClaimController) OnChange(ctx context.Context, name string, sync USBDeviceClaimHandler) {
	c.AddGenericHandler(ctx, name, FromUSBDeviceClaimHandlerToHandler(sync))
}

func (c *uSBDeviceClaimController) OnRemove(ctx context.Context, name string, sync USBDeviceClaimHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromUSBDeviceClaimHandlerToHandler(sync)))
}

func (c *uSBDeviceClaimController) Enqueue(name string) {
	c.controller.Enqueue("", name)
}

func (c *uSBDeviceClaimController) EnqueueAfter(name string, duration time.Duration) {
	c.controller.EnqueueAfter("", name, duration)
}

func (c *uSBDeviceClaimController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *uSBDeviceClaimController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *uSBDeviceClaimController) Cache() USBDeviceClaimCache {
	return &uSBDeviceClaimCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *uSBDeviceClaimController) Create(obj *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	result := &v1beta1.USBDeviceClaim{}
	return result, c.client.Create(context.TODO(), "", obj, result, metav1.CreateOptions{})
}

func (c *uSBDeviceClaimController) Update(obj *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	result := &v1beta1.USBDeviceClaim{}
	return result, c.client.Update(context.TODO(), "", obj, result, metav1.UpdateOptions{})
}

func (c *uSBDeviceClaimController) UpdateStatus(obj *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	result := &v1beta1.USBDeviceClaim{}
	return result, c.client.UpdateStatus(context.TODO(), "", obj, result, metav1.UpdateOptions{})
}

func (c *uSBDeviceClaimController) Delete(name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), "", name, *options)
}

func (c *uSBDeviceClaimController) Get(name string, options metav1.GetOptions) (*v1beta1.USBDeviceClaim, error) {
	result := &v1beta1.USBDeviceClaim{}
	return result, c.client.Get(context.TODO(), "", name, result, options)
}

func (c *uSBDeviceClaimController) List(opts metav1.ListOptions) (*v1beta1.USBDeviceClaimList, error) {
	result := &v1beta1.USBDeviceClaimList{}
	return result, c.client.List(context.TODO(), "", result, opts)
}

func (c *uSBDeviceClaimController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), "", opts)
}

func (c *uSBDeviceClaimController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*v1beta1.USBDeviceClaim, error) {
	result := &v1beta1.USBDeviceClaim{}
	return result, c.client.Patch(context.TODO(), "", name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type uSBDeviceClaimCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *uSBDeviceClaimCache) Get(name string) (*v1beta1.USBDeviceClaim, error) {
	obj, exists, err := c.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v1beta1.USBDeviceClaim), nil
}

func (c *uSBDeviceClaimCache) List(selector labels.Selector) (ret []*v1beta1.USBDeviceClaim, err error) {

	err = cache.ListAll(c.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.USBDeviceClaim))
	})

	return ret, err
}

func (c *uSBDeviceClaimCache) AddIndexer(indexName string, indexer USBDeviceClaimIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v1beta1.USBDeviceClaim))
		},
	}))
}

func (c *uSBDeviceClaimCache) GetByIndex(indexName, key string) (result []*v1beta1.USBDeviceClaim, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v1beta1.USBDeviceClaim, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v1beta1.USBDeviceClaim))
	}
	return result, nil
}

type USBDeviceClaimStatusHandler func(obj *v1beta1.USBDeviceClaim, status v1beta1.USBDeviceClaimStatus) (v1beta1.USBDeviceClaimStatus, error)

type USBDeviceClaimGeneratingHandler func(obj *v1beta1.USBDeviceClaim, status v1beta1.USBDeviceClaimStatus) ([]runtime.Object, v1beta1.USBDeviceClaimStatus, error)

func RegisterUSBDeviceClaimStatusHandler(ctx context.Context, controller USBDeviceClaimController, condition condition.Cond, name string, handler USBDeviceClaimStatusHandler) {
	statusHandler := &uSBDeviceClaimStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromUSBDeviceClaimHandlerToHandler(statusHandler.sync))
}

func RegisterUSBDeviceClaimGeneratingHandler(ctx context.Context, controller USBDeviceClaimController, apply apply.Apply,
	condition condition.Cond, name string, handler USBDeviceClaimGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &uSBDeviceClaimGeneratingHandler{
		USBDeviceClaimGeneratingHandler: handler,
		apply:                           apply,
		name:                            name,
		gvk:                             controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterUSBDeviceClaimStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type uSBDeviceClaimStatusHandler struct {
	client    USBDeviceClaimClient
	condition condition.Cond
	handler   USBDeviceClaimStatusHandler
}

func (a *uSBDeviceClaimStatusHandler) sync(key string, obj *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type uSBDeviceClaimGeneratingHandler struct {
	USBDeviceClaimGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *uSBDeviceClaimGeneratingHandler) Remove(key string, obj *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1beta1.USBDeviceClaim{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *uSBDeviceClaimGeneratingHandler) Handle(obj *v1beta1.USBDeviceClaim, status v1beta1.USBDeviceClaimStatus) (v1beta1.USBDeviceClaimStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.USBDeviceClaimGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
