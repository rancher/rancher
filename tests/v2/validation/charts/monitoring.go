package charts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/rancher/rancher/tests/v2/validation/charts/resources"
	"gopkg.in/yaml.v2"

	"github.com/pkg/errors"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/serviceaccounts"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusterrolebindings"
	"github.com/rancher/shepherd/extensions/configmaps"
	"github.com/rancher/shepherd/extensions/ingresses"
	wloads "github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubewait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Project that example app and charts are installed in
	projectName = "System"
	// Secret path that contains encoded alert manager config
	secretPath = "alertmanager.yaml"
	// Secret ID that points alert manager secret
	alertManagerSecretID = charts.RancherMonitoringNamespace + "/" + charts.RancherMonitoringAlertSecret
	// Default random string length for random name generation
	defaultRandStringLength = 5
	// Webhook deployment annotation key that is being watched
	webhookReceiverAnnotationKey = "didReceiveRequestFromAlertmanager"
	// Webhook deployment annotation value that is being watched
	webhookReceiverAnnotationValue = "true"
	// Steve type for prometheus rules for schema
	prometheusRulesSteveType = "monitoring.coreos.com.prometheusrule"
	// rancherShellSettingID is the setting ID that used to grab rancher/shell image
	rancherShellSettingID = "shell-image"
	// Kubeconfig that linked to webhook deployment
	kubeConfig = `
apiVersion: v1
kind: Config
clusters:
- name: cluster
  cluster:
    certificate-authority: /run/secrets/kubernetes.io/serviceaccount/ca.crt
    server: https://kubernetes.default
contexts:
- name: default
  context:
    cluster: cluster
    user: user
current-context: default
users:
- name: user
  user:
    tokenFile: /run/secrets/kubernetes.io/serviceaccount/token
`
)

var (
	// Rancher monitoring chart alert manager path
	alertManagerPath = "api/v1/namespaces/cattle-monitoring-system/services/http:rancher-monitoring-alertmanager:9093/proxy/#/alerts"
	// Rancher monitoring chart grafana path
	grafanaPath = "api/v1/namespaces/cattle-monitoring-system/services/http:rancher-monitoring-grafana:80/proxy"
	// Rancher monitoring chart prometheus path
	prometheusPath = "api/v1/namespaces/cattle-monitoring-system/services/http:rancher-monitoring-prometheus:9090/proxy"
	// Rancher monitoring chart prometheus graph path
	prometheusGraphPath = prometheusPath + "/graph"
	// Rancher monitoring chart prometheus rules path
	prometheusRulesPath = prometheusPath + "/rules"
	// Rancher monitoring chart prometheus targets path
	prometheusTargetsPath = prometheusPath + "/targets"
	// Rancher monitoring chart prometheus targets API path
	prometheusTargetsPathAPI = prometheusPath + "/api/v1/targets"
	// Webhook receiver kubernetes object names
	webhookReceiverNamespaceName  = "webhook-namespace-" + namegenerator.RandStringLower(defaultRandStringLength)
	webhookReceiverDeploymentName = "webhook-" + namegenerator.RandStringLower(defaultRandStringLength)
	webhookReceiverServiceName    = "webhook-service-" + namegenerator.RandStringLower(defaultRandStringLength)
	// Label that is used to identify webhook and rule
	ruleLabel = map[string]string{"team": "qa"}
)

// waitUnknownPrometheusTargets is a private helper function
// that awaits the unknown Prometheus targets to be resolved until the timeout by using Prometheus API.
func waitUnknownPrometheusTargets(client *rancher.Client) error {
	checkUnknownPrometheusTargets := func() (bool, error) {
		var statusInit bool
		var unknownTargets []string
		bodyString, err := ingresses.GetExternalIngressResponse(client, client.RancherConfig.Host, prometheusTargetsPathAPI, true)
		if err != nil {
			return statusInit, err
		}

		var mapResponse map[string]interface{}
		if err = json.Unmarshal([]byte(bodyString), &mapResponse); err != nil {
			return statusInit, err
		}
		if mapResponse["status"] != "success" {
			return statusInit, errors.New("failed to get targets from prometheus")
		}
		activeTargets := mapResponse["data"].(map[string]interface{})["activeTargets"].([]interface{})
		if len(activeTargets) < 1 {
			return false, errors.New("failed to find any active targets")
		}
		for _, target := range activeTargets {
			targetMap := target.(map[string]interface{})
			if targetMap["health"].(string) == "unknown" {
				unknownTargets = append(unknownTargets, targetMap["labels"].(map[string]interface{})["instance"].(string))
			}
		}
		return len(unknownTargets) == 0, nil
	}

	return kubewait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, 2*time.Minute, true, func(context.Context) (ongoing bool, err error) {
		result, err := checkUnknownPrometheusTargets()
		if err != nil {
			return ongoing, err
		}

		if result {
			return !ongoing, nil
		}

		return
	})
}

// checkPrometheusTargets is a private helper function
// that checks if all active prometheus targets are healthy by using prometheus API.
func checkPrometheusTargets(client *rancher.Client) (bool, error) {
	var statusInit bool
	var downTargets []string

	err := waitUnknownPrometheusTargets(client)
	if err != nil {
		return statusInit, err
	}

	bodyString, err := ingresses.GetExternalIngressResponse(client, client.RancherConfig.Host, prometheusTargetsPathAPI, true)
	if err != nil {
		return statusInit, err
	}

	var mapResponse map[string]interface{}
	if err = json.Unmarshal([]byte(bodyString), &mapResponse); err != nil {
		return statusInit, err
	}

	if mapResponse["status"] != "success" {
		return statusInit, errors.New("failed to get targets from prometheus")
	}

	activeTargets := mapResponse["data"].(map[string]interface{})["activeTargets"].([]interface{})
	if len(activeTargets) < 1 {
		return false, errors.New("failed to find any active targets")
	}

	for _, target := range activeTargets {
		targetMap := target.(map[string]interface{})
		if targetMap["health"].(string) == "down" {
			downTargets = append(downTargets, targetMap["labels"].(map[string]interface{})["instance"].(string))
		}
	}
	statusInit = len(downTargets) == 0

	if !statusInit {
		return statusInit, errors.Wrapf(err, "All active target(s) are not healthy: %v", downTargets)
	}

	return statusInit, nil
}

// editAlertReceiver is a private helper function
// that edits alert config structure to be used by the webhook receiver.
func editAlertReceiver(alertConfigByte []byte, originURL *url.URL) ([]byte, error) {
	alertConfig := &resources.AlertmanagerConfig{}
	err := yaml.Unmarshal(alertConfigByte, alertConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal alert config")
	}

	vsendresolved := false

	alertConfig.Global = &resources.GlobalConfig{
		ResolveTimeout: alertConfig.Global.ResolveTimeout,
	}
	alertConfig.Receivers = append(alertConfig.Receivers, &resources.Receiver{
		Name: webhookReceiverDeploymentName,
		WebhookConfigs: []*resources.WebhookConfig{
			{
				VSendResolved: &vsendresolved,
				HTTPConfig: &resources.HTTPClientConfig{
					ProxyURL: originURL.String(),
				},
				URL: originURL.String(),
			},
		},
	})

	byteAlertConfig, err := yaml.Marshal(alertConfig)
	if err != nil {
		return nil, err
	}

	dst := make([]byte, base64.StdEncoding.EncodedLen(len(byteAlertConfig)))
	base64.StdEncoding.Encode(dst, byteAlertConfig)

	return byteAlertConfig, nil
}

// editAlertRoute is a private helper function
// that edits alert config structure to be used by the webhook receiver.
func editAlertRoute(alertConfigByte []byte) ([]byte, error) {
	alertConfig := &resources.AlertmanagerConfig{}
	err := yaml.Unmarshal(alertConfigByte, alertConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal alert config")
	}

	alertConfig.Global = &resources.GlobalConfig{
		ResolveTimeout: alertConfig.Global.ResolveTimeout,
	}

	alertConfig.Route.Routes = append(alertConfig.Route.Routes, &resources.Route{
		Receiver:       webhookReceiverDeploymentName,
		Match:          ruleLabel,
		GroupWait:      alertConfig.Route.GroupWait,
		GroupInterval:  alertConfig.Route.GroupInterval,
		RepeatInterval: alertConfig.Route.RepeatInterval,
	})

	byteAlertConfig, err := yaml.Marshal(alertConfig)
	if err != nil {
		return nil, err
	}
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(byteAlertConfig)))
	base64.StdEncoding.Encode(dst, byteAlertConfig)

	return byteAlertConfig, nil
}

// createPrometheusRule is a private helper function
// that creates a prometheus rule to be used by the webhook receiver.
func createPrometheusRule(client *rancher.Client, clusterID string) error {
	ruleName := "webhook-rule-" + namegenerator.RandStringLower(defaultRandStringLength)
	alertName := "alert-" + namegenerator.RandStringLower(defaultRandStringLength)

	_, err := client.ReLogin()
	if err != nil {
		return err
	}

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	prometheusRule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: charts.RancherMonitoringNamespace,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: ruleName,
					Rules: []monitoringv1.Rule{
						{
							Alert:  alertName,
							Expr:   intstr.IntOrString{Type: intstr.String, StrVal: "vector(0)"},
							Labels: ruleLabel,
							For:    "0s",
						},
					},
				},
			},
		},
	}
	_, err = steveclient.SteveType(prometheusRulesSteveType).Create(prometheusRule)
	if err != nil {
		return err
	}

	return nil
}

// createWebhookReceiverDeployment is a private helper function that creates a service account, cluster role binding, and deployment for webhook receiver.
// The deployment has two different containers with a shared volume, one for kubectl commands, and the other one to receive requests and write access logs to the shared empty dir volume.
// Container that uses rancher/shell has a mounted volume to use the kubeconfig of the cluster. And it watches the access logs until a request from "alermanager" is received.
// When the request is received it sets its deployment annotation "didReceiveRequestFromAlertmanager" to "true" while the annotations being watched by the test itself.
func createAlertWebhookReceiverDeployment(client *rancher.Client, clusterID, namespace, deploymentName string) (*v1.SteveAPIObject, error) {
	serviceAccountName := "alert-receiver-sa-" + namegenerator.RandStringLower(defaultRandStringLength)
	clusterRoleBindingName := "alert-receiver-cluster-admin-" + namegenerator.RandStringLower(defaultRandStringLength)
	configMapName := "alert-receiver-cm-" + namegenerator.RandStringLower(defaultRandStringLength)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	// Create webhook receiver service account
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	_, err = steveclient.SteveType(serviceaccounts.ServiceAccountSteveType).Create(serviceAccount)
	if err != nil {
		return nil, err
	}

	// Create webhook receiver cluster role binding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}

	_, err = steveclient.SteveType(clusterrolebindings.ClusterRoleBindingSteveType).Create(clusterRoleBinding)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{}
	labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", namespace, deploymentName)

	// Create webhook receiver config map
	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"config": kubeConfig,
		},
	}

	_, err = steveclient.SteveType(configmaps.ConfigMapSteveType).Create(configmap)
	if err != nil {
		return nil, err
	}

	imageSetting, err := client.Management.Setting.ByID(rancherShellSettingID)
	if err != nil {
		return nil, err
	}

	// Create webhook receiver deployment
	var runAsUser int64
	var runAsGroup int64
	podSpecTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alert-receiver-deployment",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccount.Name,
			Containers: []corev1.Container{
				{
					Name:    "kubectl",
					Image:   imageSetting.Value,
					Command: []string{"/bin/sh", "-c"},
					Args: []string{
						fmt.Sprintf(
							`until [ "$didReceiveRequestFromAlertmanager" = true ]; do if grep -q "Alertmanager" "/traefik/access.log"; then kubectl patch deployment %s -n %s --type "json" -p '[{"op":"add","path":"/metadata/annotations/%s","value":"%s"}]'; didReceiveRequestFromAlertmanager=true; sleep 5m; else sleep 10; echo "Checking logs file one more time"; fi; done`,
							deploymentName, namespace, webhookReceiverAnnotationKey, webhookReceiverAnnotationValue,
						),
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &runAsUser,
						RunAsGroup: &runAsGroup,
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "config", MountPath: "/root/usr/share/.kube/"},
						{Name: "logs", MountPath: "/traefik"},
					},
				},
				{
					Name:  "traefik",
					Image: "traefik:latest",
					Args: []string{
						"--entrypoints.web.address=:80", "--api.dashboard=true", "--api.insecure=true", "--accesslog=true", "--accesslog.filepath=/var/log/traefik/access.log", "--log.level=INFO", "--accesslog.fields.headers.defaultmode=keep",
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
						{
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "logs", MountPath: "/var/log/traefik"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: configmap.Name},
						},
					},
				},
				{
					Name: "logs",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	isCattleLabeled := true

	deploymentTemplate := wloads.NewDeploymentTemplate(deploymentName, namespace, podSpecTemplate, isCattleLabeled, nil)
	deployment, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	if err != nil {
		return deployment, err
	}

	return deployment, nil
}
