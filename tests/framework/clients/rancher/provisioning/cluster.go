package provisioning

import (
	"context"
	"time"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	scheme "github.com/rancher/rancher/pkg/generated/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	rancherProvisioningClustersURL = "v1/provisioning.cattle.io.clusters/"
)


// Get takes name of the cluster, and returns the corresponding cluster object, and an error if there is any.
func (c *Clusters) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.Cluster, err error) {
	result = &v1.Cluster{}
	url := rancherProvisioningClustersURL + c.ns

	err = c.client.Get().
		AbsPath(url).
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Clusters that match those selectors.
func (c *Clusters) List(ctx context.Context, opts metav1.ListOptions) (result *ClusterList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	result = &ClusterList{}


	url := rancherProvisioningClustersURL + c.ns

	err = c.client.Get().
		AbsPath(url).
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Create takes the representation of a cluster and creates it. Returns the server's representation of the cluster, and an error, if there is any.
// It register its delete function to the session.Session that is being reference.
func (c *Clusters) Create(ctx context.Context, cluster *v1.Cluster, opts metav1.CreateOptions) (result *v1.Cluster, err error) {
	c.ts.RegisterCleanupFunc(func() error {
		err := c.Delete(context.TODO(), cluster.GetName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	})

	result = &v1.Cluster{}
	url := rancherProvisioningClustersURL + c.ns

	err = c.client.Post().
		AbsPath(url).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(cluster).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a cluster and updates it. Returns the server's representation of the cluster, and an error, if there is any.
func (c *Clusters) Update(ctx context.Context, cluster *v1.Cluster, opts metav1.UpdateOptions) (result *v1.Cluster, err error) {
	result = &v1.Cluster{}
	url := rancherProvisioningClustersURL + c.ns

	err = c.client.Put().
		AbsPath(url).
		Name(cluster.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(cluster).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *Clusters) UpdateStatus(ctx context.Context, cluster *v1.Cluster, opts metav1.UpdateOptions) (result *v1.Cluster, err error) {
	result = &v1.Cluster{}
	url := rancherProvisioningClustersURL + c.ns

	err = c.client.Put().
		AbsPath(url).
		Name(cluster.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(cluster).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the cluster and deletes it. Returns an error if one occurs.
func (c *Clusters) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	url := rancherProvisioningClustersURL + c.ns

	return c.client.Delete().
		AbsPath(url).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *Clusters) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	url := rancherProvisioningClustersURL + c.ns

	return c.client.Delete().
		AbsPath(url).
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched cluster.
func (c *Clusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Cluster, err error) {
	result = &v1.Cluster{}
	url := rancherProvisioningClustersURL + c.ns

	err = c.client.Patch(pt).
		AbsPath(url).
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
