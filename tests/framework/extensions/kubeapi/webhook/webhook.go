package webhook

import (
	"context"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	clientV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	V1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetWebhook is a helper function that uses the dynamic client to get a list of webhooks from a cluster
func GetWebhook(client *rancher.Client, clusterID, resourceName string) (*V1.ValidatingWebhookConfiguration, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}
	WebHookGroupVersionResource := schema.GroupVersionResource{
		Group:    "admissionregistration.k8s.io",
		Version:  "v1",
		Resource: "validatingwebhookconfigurations",
	}

	result, err := dynamicClient.Resource(WebHookGroupVersionResource).Get(context.TODO(), resourceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	webhook := &V1.ValidatingWebhookConfiguration{}
	err = clientV1.ConvertToK8sType(result.Object, webhook)
	if err != nil {
		return nil, err
	}
	return webhook, nil

}
