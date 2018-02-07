package logging

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterLoggingHandler struct {
	ClusterLoggingClient v3.ClusterLoggingInterface
	CoreV1               corev1.Interface
}

func (h *ClusterLoggingHandler) ListHandler(request *types.APIContext, _ types.RequestHandler) error {
	if request.ID != "" {
		strArray := strings.Split(request.ID, ":")
		if len(strArray) != 2 {
			return nil
		}
		clusterName := strArray[0]
		id := strArray[1]
		cl, err := h.ClusterLoggingClient.GetNamespaced(clusterName, id, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("get cluster logging by ID failed, %v", err)
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
		}
		if cl.Spec.EmbeddedConfig != nil {
			err = h.setEmbeddedEndpoint(cl)
			if err != nil {
				return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("set embedded endpoint failed, %v", err))
			}
		}

		clusterLoggingData, err := convertToClusterResource(request.Schema, *cl)
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
		}
		request.WriteResponse(http.StatusOK, clusterLoggingData)
		return nil
	}

	cls, err := h.ClusterLoggingClient.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("list cluster logging failed, %v", err)
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
	}
	clusterLoggingDatas := make([]map[string]interface{}, len(cls.Items))
	for _, cl := range cls.Items {
		if cl.Spec.EmbeddedConfig != nil {
			err = h.setEmbeddedEndpoint(&cl)
			if err != nil {
				return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
			}
		}
		clData, err := convertToClusterResource(request.Schema, cl)
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
		}
		clusterLoggingDatas = append(clusterLoggingDatas, clData)
	}
	request.WriteResponse(http.StatusOK, clusterLoggingDatas)
	return nil
}

func convertToClusterResource(schema *types.Schema, clusterLogging v3.ClusterLogging) (map[string]interface{}, error) {
	clusterLoggingData, err := convert.EncodeToMap(clusterLogging)
	if err != nil {
		return nil, err
	}
	mapper := schema.Mapper
	if mapper == nil {
		return nil, errors.New("no schema mapper available")
	}
	mapper.FromInternal(clusterLoggingData)
	return clusterLoggingData, nil
}

func (h *ClusterLoggingHandler) setEmbeddedEndpoint(cl *v3.ClusterLogging) error {
	if cl.Spec.EmbeddedConfig != nil {
		esLabels := fmt.Sprintf("%s=%s", loggingconfig.LabelK8sApp, loggingconfig.EmbeddedESName)
		espods, err := h.CoreV1.Pods(loggingconfig.LoggingNamespace).List(metav1.ListOptions{LabelSelector: esLabels})
		if err != nil {
			return err
		}
		esservice, err := h.CoreV1.Services(loggingconfig.LoggingNamespace).Get(loggingconfig.EmbeddedESName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(esservice.Spec.Ports) == 0 {
			return fmt.Errorf("get service %s node port failed", loggingconfig.EmbeddedKibanaName)
		}
		var esPort int32
		for _, v := range esservice.Spec.Ports {
			if v.Name == "http" {
				esPort = v.NodePort
				break
			}
		}

		if len(espods.Items) > 0 && espods.Items[0].Status.Phase == "Running" {
			cl.Spec.EmbeddedConfig.ElasticsearchEndpoint = fmt.Sprintf("%s:%v", espods.Items[0].Status.HostIP, esPort)
		}

		kibanaLabels := fmt.Sprintf("%s=%s", loggingconfig.LabelK8sApp, loggingconfig.EmbeddedKibanaName)
		kibanapods, err := h.CoreV1.Pods(loggingconfig.LoggingNamespace).List(metav1.ListOptions{LabelSelector: kibanaLabels})
		if err != nil {
			return err
		}
		kibanaservice, err := h.CoreV1.Services(loggingconfig.LoggingNamespace).Get(loggingconfig.EmbeddedKibanaName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(kibanaservice.Spec.Ports) == 0 {
			return fmt.Errorf("get service %s node port failed", loggingconfig.EmbeddedKibanaName)
		}

		if len(kibanapods.Items) > 0 && kibanapods.Items[0].Status.Phase == "Running" {
			cl.Spec.EmbeddedConfig.KibanaEndpoint = fmt.Sprintf("%s:%v", kibanapods.Items[0].Status.HostIP, kibanaservice.Spec.Ports[0].NodePort)
		}
	}
	return nil
}
