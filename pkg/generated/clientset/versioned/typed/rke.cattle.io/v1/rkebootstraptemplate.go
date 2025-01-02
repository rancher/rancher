/*
Copyright 2025 Rancher Labs, Inc.

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

package v1

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	scheme "github.com/rancher/rancher/pkg/generated/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// RKEBootstrapTemplatesGetter has a method to return a RKEBootstrapTemplateInterface.
// A group's client should implement this interface.
type RKEBootstrapTemplatesGetter interface {
	RKEBootstrapTemplates(namespace string) RKEBootstrapTemplateInterface
}

// RKEBootstrapTemplateInterface has methods to work with RKEBootstrapTemplate resources.
type RKEBootstrapTemplateInterface interface {
	Create(ctx context.Context, rKEBootstrapTemplate *v1.RKEBootstrapTemplate, opts metav1.CreateOptions) (*v1.RKEBootstrapTemplate, error)
	Update(ctx context.Context, rKEBootstrapTemplate *v1.RKEBootstrapTemplate, opts metav1.UpdateOptions) (*v1.RKEBootstrapTemplate, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.RKEBootstrapTemplate, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.RKEBootstrapTemplateList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RKEBootstrapTemplate, err error)
	RKEBootstrapTemplateExpansion
}

// rKEBootstrapTemplates implements RKEBootstrapTemplateInterface
type rKEBootstrapTemplates struct {
	*gentype.ClientWithList[*v1.RKEBootstrapTemplate, *v1.RKEBootstrapTemplateList]
}

// newRKEBootstrapTemplates returns a RKEBootstrapTemplates
func newRKEBootstrapTemplates(c *RkeV1Client, namespace string) *rKEBootstrapTemplates {
	return &rKEBootstrapTemplates{
		gentype.NewClientWithList[*v1.RKEBootstrapTemplate, *v1.RKEBootstrapTemplateList](
			"rkebootstraptemplates",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *v1.RKEBootstrapTemplate { return &v1.RKEBootstrapTemplate{} },
			func() *v1.RKEBootstrapTemplateList { return &v1.RKEBootstrapTemplateList{} }),
	}
}
