// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"encoding/json"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	dynamic "k8s.io/client-go/deprecated-dynamic"
	"k8s.io/client-go/rest"
)

const (
	PrometheusesKind = "Prometheus"
	PrometheusName   = "prometheuses"
)

type PrometheusesGetter interface {
	Prometheuses(namespace string) PrometheusInterface
}

var _ PrometheusInterface = &prometheuses{}

type PrometheusInterface interface {
	Create(*Prometheus) (*Prometheus, error)
	Get(name string, opts metav1.GetOptions) (*Prometheus, error)
	Update(*Prometheus) (*Prometheus, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error
}

type prometheuses struct {
	restClient rest.Interface
	client     dynamic.ResourceInterface
	crdKind    CrdKind
	ns         string
}

func newPrometheuses(r rest.Interface, c *dynamic.Client, crdKind CrdKind, namespace string) *prometheuses {
	return &prometheuses{
		restClient: r,
		client: c.Resource(
			&metav1.APIResource{
				Kind:       crdKind.Kind,
				Name:       crdKind.Plural,
				Namespaced: true,
			},
			namespace,
		),
		crdKind: crdKind,
		ns:      namespace,
	}
}

func (p *prometheuses) Create(o *Prometheus) (*Prometheus, error) {
	up, err := UnstructuredFromPrometheus(o)
	if err != nil {
		return nil, err
	}

	up, err = p.client.Create(up)
	if err != nil {
		return nil, err
	}

	return PrometheusFromUnstructured(up)
}

func (p *prometheuses) Get(name string, opts metav1.GetOptions) (*Prometheus, error) {
	obj, err := p.client.Get(name, opts)
	if err != nil {
		return nil, err
	}
	return PrometheusFromUnstructured(obj)
}

func (p *prometheuses) Update(o *Prometheus) (*Prometheus, error) {
	up, err := UnstructuredFromPrometheus(o)
	if err != nil {
		return nil, err
	}

	curp, err := p.Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current version for update")
	}
	up.SetResourceVersion(curp.ObjectMeta.ResourceVersion)

	up, err = p.client.Update(up)
	if err != nil {
		return nil, err
	}

	return PrometheusFromUnstructured(up)
}

func (p *prometheuses) Delete(name string, options *metav1.DeleteOptions) error {
	return p.client.Delete(name, options)
}

func (p *prometheuses) List(opts metav1.ListOptions) (runtime.Object, error) {
	req := p.restClient.Get().
		Namespace(p.ns).
		Resource(p.crdKind.Plural)

	b, err := req.DoRaw()
	if err != nil {
		return nil, err
	}
	var prom PrometheusList
	return &prom, json.Unmarshal(b, &prom)
}

func (p *prometheuses) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := p.restClient.Get().
		Prefix("watch").
		Namespace(p.ns).
		Resource(p.crdKind.Plural).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&prometheusDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil
}

func (p *prometheuses) DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error {
	return p.client.DeleteCollection(dopts, lopts)
}

// PrometheusFromUnstructured unmarshals a Prometheus object from dynamic client's unstructured
func PrometheusFromUnstructured(r *unstructured.Unstructured) (*Prometheus, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var p Prometheus
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	p.TypeMeta.Kind = PrometheusesKind
	p.TypeMeta.APIVersion = Group + "/" + Version
	return &p, nil
}

// UnstructuredFromPrometheus marshals a Prometheus object into dynamic client's unstructured
func UnstructuredFromPrometheus(p *Prometheus) (*unstructured.Unstructured, error) {
	p.TypeMeta.Kind = PrometheusesKind
	p.TypeMeta.APIVersion = Group + "/" + Version
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var r unstructured.Unstructured
	if err := json.Unmarshal(b, &r.Object); err != nil {
		return nil, err
	}
	// Value-type timestamp fields like ObjectMeta.CreationTimestamp with a zero
	// value are marshalled as "null" in JSON (rather than omitted) and then
	// unmarshalled into Unstructured with the key intact and a null value (rather
	// than being omitted); the net effect is the resulting structs can't be used
	// to issue a POST because creationTimestamp=null is sent to the server and
	// fails validation. For example, passing a Prometheus with a
	// volumeClaimTemplate can result in an invalid object. This hack simply
	// removes such timestamp fields manually.
	//
	// TODO: reevaluate the use of Unstructured directly here in the context of
	// the latest dynamic client capabilities; this manual conversion may not be
	// necessary anymore.
	unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "spec", "storage", "volumeClaimTemplate", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "spec", "podMetadata", "creationTimestamp")
	return &r, nil
}

type prometheusDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *prometheusDecoder) Close() {
	d.close()
}

func (d *prometheusDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object Prometheus
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
