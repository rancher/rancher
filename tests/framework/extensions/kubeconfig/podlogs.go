package kubeconfig

import (
	"bufio"
	"context"
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8Scheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
)

// GetPodLogs is a helper function that returns logs from a pod using rest client
func GetPodLogs(client *rancher.Client, clusterID string, podName string, namespace string) (string, error) {
	var restConfig *restclient.Config

	kubeConfig, err := GetKubeconfig(client, clusterID)
	if err != nil {
		return "", err
	}

	restConfig, err = (*kubeConfig).ClientConfig()
	if err != nil {
		return "", err
	}
	restConfig.ContentConfig.NegotiatedSerializer = serializer.NewCodecFactory(k8Scheme.Scheme)
	restConfig.ContentConfig.GroupVersion = &podGroupVersion
	restConfig.APIPath = apiPath

	restClient, err := restclient.RESTClientFor(restConfig)
	if err != nil {
		return "", err
	}

	req := restClient.Get().Resource("pods").Name(podName).Namespace(namespace).SubResource("log")
	option := &corev1.PodLogOptions{}
	req.VersionedParams(
		option,
		k8Scheme.ParameterCodec,
	)

	stream, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error streaming pod logs for pod %s/%s: %v", namespace, podName, err)
	}

	defer stream.Close()

	reader := bufio.NewScanner(stream)
	var logs string
	for reader.Scan() {
		logs = logs + fmt.Sprintf("%s\n", reader.Text())
	}

	if err := reader.Err(); err != nil {
		return "", fmt.Errorf("error reading pod logs for pod %s/%s: %v", namespace, podName, err)
	}
	return logs, nil
}

func GetPodLogsWithOpts(client *rancher.Client, clusterID string, podName string, namespace string, opts *corev1.PodLogOptions) (string, error) {
	var restConfig *restclient.Config

	kubeConfig, err := GetKubeconfig(client, clusterID)
	if err != nil {
		return "", err
	}

	restConfig, err = (*kubeConfig).ClientConfig()
	if err != nil {
		return "", err
	}
	restConfig.ContentConfig.NegotiatedSerializer = serializer.NewCodecFactory(k8Scheme.Scheme)
	restConfig.ContentConfig.GroupVersion = &podGroupVersion
	restConfig.APIPath = apiPath

	restClient, err := restclient.RESTClientFor(restConfig)
	if err != nil {
		return "", err
	}

	req := restClient.Get().Resource("pods").Name(podName).Namespace(namespace).SubResource("log")
	req.VersionedParams(
		opts,
		k8Scheme.ParameterCodec,
	)

	stream, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error streaming pod logs for pod %s/%s: %v", namespace, podName, err)
	}

	defer stream.Close()

	reader := bufio.NewScanner(stream)
	var logs string
	for reader.Scan() {
		logs = logs + fmt.Sprintf("%s\n", reader.Text())
		fmt.Println(reader.Text())
	}

	if err := reader.Err(); err != nil {
		return "", fmt.Errorf("error reading pod logs for pod %s/%s: %v", namespace, podName, err)
	}
	return logs, nil
}
