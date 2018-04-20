package approuter

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/ticker"

	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/settings"
	rkek8s "github.com/rancher/rke/k8s"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	nginxIngressLabels = map[string]string{
		"app": "ingress-nginx",
	}
	renewInterval = 5 * time.Minute
)

type NginxIngressController struct {
	workloadController workloadutil.CommonController
	nodeLister         v1.NodeLister
	podLister          v1.PodLister
	dnsClient          *Client
	clusterName        string
	ingressController  v1beta1.IngressController
}

func (n *NginxIngressController) sync(key string, obj *workloadutil.Workload) error {
	serverURL := settings.BaseRDNSServerURL.Get()
	if serverURL == "" { //rootDomain and serverURL need to be set when enable rdns controller
		return nil
	}

	if !labels.SelectorFromSet(nginxIngressLabels).Matches(labels.Set(obj.Labels)) {
		return nil
	}

	ips, err := n.getNginxControllerIPs()
	if err != nil {
		return err
	}
	if serverURL != n.dnsClient.GetBaseURL() {
		n.dnsClient.SetBaseURL(serverURL)
	}

	fqdn, err := n.dnsClient.ApplyDomain(ips)
	if err != nil {
		logrus.WithError(err).Errorf("update fqdn [%s] to server [%s] error", fqdn, serverURL)
		return err
	}
	logrus.Infof("update fqdn [%s] to server [%s] success", fqdn, serverURL)
	n.ingressController.Enqueue("", refreshIngressHostnameKey)
	return nil
}

func (n *NginxIngressController) getNginxControllerIPs() ([]string, error) {
	var ips []string
	pods, err := n.podLister.List(defaultNginxIngressNamespace, labels.SelectorFromSet(labels.Set(nginxIngressLabels)))
	if err != nil {
		logrus.WithError(err).Error("syncing ingress rules to rdns server error")
		return nil, err
	}
	for _, pod := range pods {
		ip, err := n.getNodePublicIP(pod.Spec.NodeName)
		if err != nil {
			logrus.WithError(err).Errorf("get node %s public ip error", pod.Spec.NodeName)
			continue
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

func (n *NginxIngressController) getNodePublicIP(nodeName string) (string, error) {
	node, err := n.nodeLister.Get("", nodeName)
	if err != nil {
		return "", err
	}
	//from ips
	ip := ""
	for _, address := range node.Status.Addresses {
		if address.Type == "ExternalIP" {
			return address.Address, nil
		}
		if address.Type == "InternalIP" {
			ip = address.Address
		}
	}

	//from annotation
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	if ip, ok := node.Annotations[rkek8s.ExternalAddressAnnotation]; ok {
		return ip, nil
	}

	if ip, ok := node.Annotations[rkek8s.InternalAddressAnnotation]; ok {
		return ip, nil
	}

	return ip, nil
}

func (n *NginxIngressController) renew(ctx context.Context) {
	for range ticker.Context(ctx, renewInterval) {
		serverURL := settings.BaseRDNSServerURL.Get()
		if serverURL == "" { //rootDomain and serverURL need to be set when enable rdns controller
			return
		}
		if serverURL != n.dnsClient.GetBaseURL() {
			n.dnsClient.SetBaseURL(serverURL)
		}
		if fqdn, err := n.dnsClient.RenewDomain(); err != nil {
			logrus.WithError(err).Errorf("renew fqdn [%s] to server [%s] error", fqdn, serverURL)
		}
	}
}
