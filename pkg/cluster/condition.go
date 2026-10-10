package cluster

import (
	"fmt"

	"github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// StatusClient is the subset of a management cluster client needed to update a cluster's status. Both
// the norman and the wrangler clients satisfy it.
type StatusClient interface {
	Get(name string, opts metav1.GetOptions) (*apimgmtv3.Cluster, error)
	UpdateStatus(*apimgmtv3.Cluster) (*apimgmtv3.Cluster, error)
}

// SetCondition sets cond on the latest version of cluster, retrying on conflicts. It refuses to update a
// different cluster that has since taken the same name, and skips the update if the condition already
// has the given status, reason and message.
func SetCondition(client StatusClient, cluster *apimgmtv3.Cluster, cond condition.Cond, status corev1.ConditionStatus, reason, message string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := client.Get(cluster.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if latest.UID != cluster.UID {
			return fmt.Errorf("cluster %s was replaced (uid %s, expected %s)", cluster.Name, latest.UID, cluster.UID)
		}
		if cond.GetStatus(latest) == string(status) && cond.GetReason(latest) == reason && cond.GetMessage(latest) == message {
			return nil
		}
		cond.SetStatus(latest, string(status))
		cond.Reason(latest, reason)
		cond.Message(latest, message)
		_, err = client.UpdateStatus(latest)
		return err
	})
}

// ConditionConcluded reports whether cond has been set to True or False on cluster.
func ConditionConcluded(cluster *apimgmtv3.Cluster, cond condition.Cond) bool {
	return cond.IsTrue(cluster) || cond.IsFalse(cluster)
}
