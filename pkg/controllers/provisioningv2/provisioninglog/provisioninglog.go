package provisioninglog

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterindex"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	provisioningLogName = "provisioning-log"
	maxLen              = 10000
)

var (
	clusterRegexp = regexp.MustCompile("^c-m-[a-z0-9]{8}$")
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		configMapsCache: clients.Core.ConfigMap().Cache(),
		configMaps:      clients.Core.ConfigMap(),
		clusterCache:    clients.Provisioning.Cluster().Cache(),
	}

	clients.Core.Namespace().OnChange(ctx, "prov-log-namespace", h.OnNamespace)
	clients.Core.ConfigMap().OnChange(ctx, "prov-log-configmap", h.OnConfigMap)
}

type handler struct {
	configMapsCache corev1controllers.ConfigMapCache
	configMaps      corev1controllers.ConfigMapController
	clusterCache    provisioningcontrollers.ClusterCache
}

func (h *handler) OnConfigMap(_ string, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if cm == nil || !cm.DeletionTimestamp.IsZero() {
		return nil, nil
	}
	if cm.Name != provisioningLogName || (!clusterRegexp.MatchString(cm.Namespace) && cm.Namespace != "local") {
		return cm, nil
	}
	provCluster, err := h.clusterCache.GetByIndex(clusterindex.ClusterV1ByClusterV3Reference, cm.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return cm, err
	}
	if apierrors.IsNotFound(err) || len(provCluster) == 0 {
		h.configMaps.EnqueueAfter(cm.Namespace, cm.Name, 2*time.Second)
		return cm, nil
	}
	if provCluster[0].Spec.RKEConfig == nil {
		return cm, nil
	}

	h.configMaps.EnqueueAfter(cm.Namespace, cm.Name, 2*time.Second)
	return h.recordMessage(provCluster[0], cm)
}

// appendLog appends a message to the provisioning log, adding a newline to the new log message, a newline to the
// previous log message (if it is not present), and trimming previous lines from the log if it exceeds the maximum
// length.
func appendLog(log, msg string) string {
	// Although it is unlikely that the provisioning log does not end with a newline (except for a brand new cluster),
	// it is better to err on the side of caution since this could cause log messages to become unintentionally combined.
	if !strings.HasSuffix(log, "\n") && log != "" {
		log += "\n"
	}
	log += msg + "\n"

	// Remove the oldest lines until log is within size limit.
	for len(log) > maxLen {
		log = strings.TrimLeftFunc(log, func(r rune) bool {
			return r != '\n'
		})
		log = strings.TrimPrefix(log, "\n")
	}
	return log
}

func (h *handler) recordMessage(cluster *provv1.Cluster, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	msg := capr.Provisioned.GetMessage(cluster)
	failure := capr.Provisioned.IsFalse(cluster)
	done := capr.Provisioned.IsTrue(cluster)

	if done && msg == "" {
		done = capr.Updated.IsTrue(cluster)
		msg = capr.Updated.GetMessage(cluster)
		failure = capr.Updated.IsFalse(cluster)
	}

	if done && msg == "" && cluster.Status.Ready {
		msg = "provisioning done"
	}

	if msg == "" {
		return cm, nil
	}

	if strings.Contains(msg, "the object has been modified; please apply your changes to the latest version and try again") {
		msg = fmt.Sprintf("Transient error encountered: %s", msg)
	}

	last := cm.Data["last"]
	if msg == last {
		return cm, nil
	}

	cm = cm.DeepCopy()
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}

	prefix := " [INFO ] "
	if failure {
		prefix = " [ERROR] "
	}

	cm.Data["log"] = appendLog(cm.Data["log"], time.Now().Format(time.RFC3339)+prefix+msg)
	cm.Data["last"] = msg
	return h.configMaps.Update(cm)
}

func (h *handler) OnNamespace(_ string, ns *corev1.Namespace) (*corev1.Namespace, error) {
	if ns == nil || !ns.DeletionTimestamp.IsZero() {
		return nil, nil
	}
	if !clusterRegexp.MatchString(ns.Name) {
		return ns, nil
	}
	if _, err := h.configMapsCache.Get(ns.Name, provisioningLogName); apierrors.IsNotFound(err) {
		_, err := h.configMaps.Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      provisioningLogName,
				Namespace: ns.Name,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("creating %s for %s: %w", provisioningLogName, ns.Name, err)
		}
	}
	return ns, nil
}
