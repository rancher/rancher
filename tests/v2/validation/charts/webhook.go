package charts

import (
	"strings"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/kubeapi/webhook"
)

const (
	resourceName    = "rancher.cattle.io"
	restrictedAdmin = "restricted-admin"
	admin           = "admin"
	localCluster    = "local"
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

func validateWebhookPodLogs(podLogs string) interface{} {

	delimiter := "\n"
	segments := strings.Split(podLogs, delimiter)

	for _, segment := range segments {
		if strings.Contains(segment, "level=error") {
			return "Error logs in webhook" + segment
		}
	}
	return nil
}
