/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	cisv1 "github.com/F5Networks/k8s-bigip-ctlr/v3/config/apis/cis/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeDeployConfigs implements DeployConfigInterface
type FakeDeployConfigs struct {
	Fake *FakeCisV1
	ns   string
}

var deployconfigsResource = schema.GroupVersionResource{Group: "cis.f5.com", Version: "v1", Resource: "deployconfigs"}

var deployconfigsKind = schema.GroupVersionKind{Group: "cis.f5.com", Version: "v1", Kind: "DeployConfig"}

// Get takes name of the deployConfig, and returns the corresponding deployConfig object, and an error if there is any.
func (c *FakeDeployConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *cisv1.DeployConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(deployconfigsResource, c.ns, name), &cisv1.DeployConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*cisv1.DeployConfig), err
}

// List takes label and field selectors, and returns the list of DeployConfigs that match those selectors.
func (c *FakeDeployConfigs) List(ctx context.Context, opts v1.ListOptions) (result *cisv1.DeployConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(deployconfigsResource, deployconfigsKind, c.ns, opts), &cisv1.DeployConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &cisv1.DeployConfigList{ListMeta: obj.(*cisv1.DeployConfigList).ListMeta}
	for _, item := range obj.(*cisv1.DeployConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested deployConfigs.
func (c *FakeDeployConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(deployconfigsResource, c.ns, opts))

}

// Create takes the representation of a deployConfig and creates it.  Returns the server's representation of the deployConfig, and an error, if there is any.
func (c *FakeDeployConfigs) Create(ctx context.Context, deployConfig *cisv1.DeployConfig, opts v1.CreateOptions) (result *cisv1.DeployConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(deployconfigsResource, c.ns, deployConfig), &cisv1.DeployConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*cisv1.DeployConfig), err
}

// Update takes the representation of a deployConfig and updates it. Returns the server's representation of the deployConfig, and an error, if there is any.
func (c *FakeDeployConfigs) Update(ctx context.Context, deployConfig *cisv1.DeployConfig, opts v1.UpdateOptions) (result *cisv1.DeployConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(deployconfigsResource, c.ns, deployConfig), &cisv1.DeployConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*cisv1.DeployConfig), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeDeployConfigs) UpdateStatus(ctx context.Context, deployConfig *cisv1.DeployConfig, opts v1.UpdateOptions) (*cisv1.DeployConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(deployconfigsResource, "status", c.ns, deployConfig), &cisv1.DeployConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*cisv1.DeployConfig), err
}

// Delete takes name of the deployConfig and deletes it. Returns an error if one occurs.
func (c *FakeDeployConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(deployconfigsResource, c.ns, name), &cisv1.DeployConfig{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDeployConfigs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(deployconfigsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &cisv1.DeployConfigList{})
	return err
}

// Patch applies the patch and returns the patched deployConfig.
func (c *FakeDeployConfigs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *cisv1.DeployConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(deployconfigsResource, c.ns, name, pt, data, subresources...), &cisv1.DeployConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*cisv1.DeployConfig), err
}
