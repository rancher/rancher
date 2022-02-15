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

package fake

import (
	"context"

	upgradecattleiov1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePlans implements PlanInterface
type FakePlans struct {
	Fake *FakeUpgradeV1
	ns   string
}

var plansResource = schema.GroupVersionResource{Group: "upgrade.cattle.io", Version: "v1", Resource: "plans"}

var plansKind = schema.GroupVersionKind{Group: "upgrade.cattle.io", Version: "v1", Kind: "Plan"}

// Get takes name of the plan, and returns the corresponding plan object, and an error if there is any.
func (c *FakePlans) Get(ctx context.Context, name string, options v1.GetOptions) (result *upgradecattleiov1.Plan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(plansResource, c.ns, name), &upgradecattleiov1.Plan{})

	if obj == nil {
		return nil, err
	}
	return obj.(*upgradecattleiov1.Plan), err
}

// List takes label and field selectors, and returns the list of Plans that match those selectors.
func (c *FakePlans) List(ctx context.Context, opts v1.ListOptions) (result *upgradecattleiov1.PlanList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(plansResource, plansKind, c.ns, opts), &upgradecattleiov1.PlanList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &upgradecattleiov1.PlanList{ListMeta: obj.(*upgradecattleiov1.PlanList).ListMeta}
	for _, item := range obj.(*upgradecattleiov1.PlanList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested plans.
func (c *FakePlans) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(plansResource, c.ns, opts))

}

// Create takes the representation of a plan and creates it.  Returns the server's representation of the plan, and an error, if there is any.
func (c *FakePlans) Create(ctx context.Context, plan *upgradecattleiov1.Plan, opts v1.CreateOptions) (result *upgradecattleiov1.Plan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(plansResource, c.ns, plan), &upgradecattleiov1.Plan{})

	if obj == nil {
		return nil, err
	}
	return obj.(*upgradecattleiov1.Plan), err
}

// Update takes the representation of a plan and updates it. Returns the server's representation of the plan, and an error, if there is any.
func (c *FakePlans) Update(ctx context.Context, plan *upgradecattleiov1.Plan, opts v1.UpdateOptions) (result *upgradecattleiov1.Plan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(plansResource, c.ns, plan), &upgradecattleiov1.Plan{})

	if obj == nil {
		return nil, err
	}
	return obj.(*upgradecattleiov1.Plan), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePlans) UpdateStatus(ctx context.Context, plan *upgradecattleiov1.Plan, opts v1.UpdateOptions) (*upgradecattleiov1.Plan, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(plansResource, "status", c.ns, plan), &upgradecattleiov1.Plan{})

	if obj == nil {
		return nil, err
	}
	return obj.(*upgradecattleiov1.Plan), err
}

// Delete takes name of the plan and deletes it. Returns an error if one occurs.
func (c *FakePlans) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(plansResource, c.ns, name, opts), &upgradecattleiov1.Plan{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePlans) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(plansResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &upgradecattleiov1.PlanList{})
	return err
}

// Patch applies the patch and returns the patched plan.
func (c *FakePlans) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *upgradecattleiov1.Plan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(plansResource, c.ns, name, pt, data, subresources...), &upgradecattleiov1.Plan{})

	if obj == nil {
		return nil, err
	}
	return obj.(*upgradecattleiov1.Plan), err
}
