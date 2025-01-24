package configserver

import (
	"bytes"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterindex"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// connectClusterAgentYAML renders a cattle-cluster-agent manifest for the given bearer token
func (r *RKE2ConfigServer) connectClusterAgentYAML(rw http.ResponseWriter, req *http.Request) {
	token := mux.Vars(req)["token"]
	if token == "" {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	tokens, err := r.clusterTokenCache.GetByIndex(tokenIndex, token)
	if err != nil {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		logrus.Errorf("[rke2configserver] error retrieving cluster token by index: %v", err)
		return
	}

	if len(tokens) == 0 {
		logrus.Errorf("[rke2configserver] no tokens found in index")
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	mgmtCluster, err := r.mgmtClusterCache.Get(tokens[0].ObjClusterName())
	if err != nil {
		logrus.Errorf("[rke2configserver] error retrieving management cluster for cluster registration token: %v", err)
		http.Error(rw, "unknown", http.StatusInternalServerError)
		return
	}

	provisioningClusters, err := r.provisioningClusterCache.GetByIndex(clusterindex.ClusterV1ByClusterV3Reference, tokens[0].ObjClusterName())
	if err != nil {
		logrus.Errorf("[rke2configserver] error retrieving provisioning cluster: %v", err)
		http.Error(rw, "unknown", http.StatusInternalServerError)
		return
	}
	// ensure this is a v2prov cluster

	if len(provisioningClusters) != 1 {
		logrus.Errorf("[rke2configserver] multiple provisioning clusters found")
		http.Error(rw, "unknown", http.StatusInternalServerError)
		return
	}

	machines, err := r.machineCache.List(provisioningClusters[0].Namespace, labels.SelectorFromSet(map[string]string{
		capi.ClusterNameLabel: provisioningClusters[0].Name,
	}))
	if err != nil {
		logrus.Errorf("[rke2configserver] error listing machines: %v", err)
		http.Error(rw, "unknown", http.StatusInternalServerError)
		return
	}

	var taints []corev1.Taint
	taintMap := make(map[corev1.Taint]bool)

	for _, m := range machines {
		machineTaints, err := capr.GetTaints(m.Annotations[capr.TaintsAnnotation], capr.GetRuntime(provisioningClusters[0].Spec.KubernetesVersion), m.Labels[capr.ControlPlaneRoleLabel] == "true", m.Labels[capr.EtcdRoleLabel] == "true", m.Labels[capr.WorkerRoleLabel] == "true")
		if err != nil {
			logrus.Errorf("[rke2configserver] error retrieving taints for machine: %v", err)
			continue
		}
		for _, t := range machineTaints {
			if _, ok := taintMap[t]; !ok {
				taints = append(taints, t)
				taintMap[t] = true
			}
		}
	}

	sort.Slice(taints, func(i, j int) bool {
		return taints[i].ToString() < taints[j].ToString()
	})

	serverURL := settings.ServerURL.Get()
	if serverURL == "" {
		logrus.Errorf("[rke2configserver] server-url not set")
		http.Error(rw, "server url not set", http.StatusInternalServerError)
		return
	}

	clusterAgentTemplate := &bytes.Buffer{}
	err = systemtemplate.SystemTemplate(clusterAgentTemplate, systemtemplate.GetDesiredAgentImage(mgmtCluster),
		systemtemplate.GetDesiredAuthImage(mgmtCluster),
		mgmtCluster.Name, token, serverURL, mgmtCluster.Spec.WindowsPreferedCluster, capr.PreBootstrap(mgmtCluster),
		mgmtCluster, systemtemplate.GetDesiredFeatures(mgmtCluster), taints, r.secretsCache)
	if err != nil {
		logrus.Errorf("[rke2configserver] error encountered rendering cluster-agent manifest: %v", err)
		http.Error(rw, "unable to render manifest", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "text/plain")
	_, err = rw.Write(clusterAgentTemplate.Bytes())
	if err != nil {
		logrus.Errorf("[rke2configserver] error while writing cluster agent YAML manifest: %v", err)
	}
	return
}
