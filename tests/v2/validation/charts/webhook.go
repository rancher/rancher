package charts

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/webhook"
)

const (
	resourceName    = "rancher.cattle.io"
	restrictedAdmin = "restricted-admin"
	admin           = "admin"
)

func getWebhookNames(client *rancher.Client, clusterID, resourceName string) ([]string, error) {
	webhookList, err := webhook.GetWebhook(client, clusterID, resourceName)
	if err != nil {
		return nil, err
	}

	var webhookL []string
	for _, webhook := range webhookList.Webhooks {
		webhookL = append(webhookL, webhook.Name)
	}

	return webhookL, nil

}
