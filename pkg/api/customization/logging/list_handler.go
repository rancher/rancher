package logging

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

const (
	failed  = "Failed"
	running = "Running"
	pending = "Pending"
)

type ClusterLoggingHandler struct {
	ClusterLoggingLister v3.ClusterLoggingLister
	PodLister            corev1.PodLister
	ServiceLister        corev1.ServiceLister
}

func (h *ClusterLoggingHandler) ListHandler(request *types.APIContext, _ types.RequestHandler) error {
	if request.ID != "" {
		strArray := strings.Split(request.ID, ":")
		if len(strArray) != 2 {
			return nil
		}
		clusterName := strArray[0]
		id := strArray[1]
		cl, err := h.ClusterLoggingLister.Get(clusterName, id)
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
		}
		if cl.Spec.EmbeddedConfig != nil {
			if err = h.setEmbeddedEndpoint(cl); err != nil {
				cl.Spec.EmbeddedConfig.ElasticsearchEndpoint = failed
				cl.Spec.EmbeddedConfig.KibanaEndpoint = failed
				logrus.Warnf("set embedded endpoint failed, %v", err)
			}
		}

		clusterLoggingData, err := convertToClusterResource(request.Schema, cl)
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
		}
		request.WriteResponse(http.StatusOK, clusterLoggingData)
		return nil
	}

	cls, err := h.ClusterLoggingLister.List("", labels.NewSelector())
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("%v", err))
	}
	clusterLoggingDatas := make([]map[string]interface{}, len(cls))
	for _, cl := range cls {
		if cl.Spec.EmbeddedConfig != nil {
			if err = h.setEmbeddedEndpoint(cl); err != nil {
				cl.Spec.EmbeddedConfig.ElasticsearchEndpoint = failed
				cl.Spec.EmbeddedConfig.KibanaEndpoint = failed
				logrus.Warnf("set embedded endpoint failed, %v", err)
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

func convertToClusterResource(schema *types.Schema, clusterLogging *v3.ClusterLogging) (map[string]interface{}, error) {
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

	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(loggingconfig.LabelK8sApp, selection.Equals, []string{loggingconfig.EmbeddedESName})
	if err != nil {
		return err
	}
	espods, err := h.PodLister.List(loggingconfig.LoggingNamespace, selector.Add(*requirement))
	if err != nil {
		return err
	}
	esservice, err := h.ServiceLister.Get(loggingconfig.LoggingNamespace, loggingconfig.EmbeddedESName)
	if err != nil {
		return err
	}
	if len(esservice.Spec.Ports) == 0 {
		return fmt.Errorf("get service %s node port failed", loggingconfig.EmbeddedESName)
	}
	var esPort int32
	for _, v := range esservice.Spec.Ports {
		if v.Name == "http" {
			esPort = v.NodePort
			break
		}
	}

	if len(espods) == 0 {
		cl.Spec.EmbeddedConfig.ElasticsearchEndpoint = pending
	} else {
		espod := espods[0]
		if espod.Status.Phase == running {
			cl.Spec.EmbeddedConfig.ElasticsearchEndpoint = fmt.Sprintf("%s:%v", espod.Status.HostIP, esPort)
		} else {
			cl.Spec.EmbeddedConfig.ElasticsearchEndpoint = fmt.Sprintf("%s", espod.Status.Phase)
		}
	}

	selector = labels.NewSelector()
	requirement, err = labels.NewRequirement(loggingconfig.LabelK8sApp, selection.Equals, []string{loggingconfig.EmbeddedKibanaName})
	if err != nil {
		return err
	}
	kibanapods, err := h.PodLister.List(loggingconfig.LoggingNamespace, selector.Add(*requirement))
	if err != nil {
		return err
	}
	kibanaservice, err := h.ServiceLister.Get(loggingconfig.LoggingNamespace, loggingconfig.EmbeddedKibanaName)
	if err != nil {
		return err
	}
	if len(kibanaservice.Spec.Ports) == 0 {
		return fmt.Errorf("get service %s node port failed", loggingconfig.EmbeddedKibanaName)
	}

	if len(kibanapods) == 0 {
		cl.Spec.EmbeddedConfig.KibanaEndpoint = pending
	} else {
		kibanapod := kibanapods[0]
		if kibanapod.Status.Phase == running {
			cl.Spec.EmbeddedConfig.KibanaEndpoint = fmt.Sprintf("%s:%v", kibanapod.Status.HostIP, esPort)
		} else {
			cl.Spec.EmbeddedConfig.KibanaEndpoint = fmt.Sprintf("%s", kibanapod.Status.Phase)
		}
	}
	return nil
}
