# What are these files?

These are Go structs for Istio CRD. We translated them from proto files in
https://github.com/istio/api/tree/master/networking/v1alpha3 .

# Why do we hand-translate from proto? i.e Why can't we vendor these?

Istio needs to run on many platforms and as a reason they represent their
objects internally as proto. On Kubernetes, their API take in JSON objects and
convert to proto before processing them.

So they have nothing we can vendor, except for the Go files that are generated
by the proto compiler, which is not compatible with K8s API code-generator at
all.

We may be able to donate our translation so they can maintain it themselves. See
https://github.com/istio/istio/issues/6084.
