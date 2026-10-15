package cluster

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

var Conditions = []string{
	string(apimgmtv3.ClusterConditionProvisioned),
	string(apimgmtv3.ClusterConditionPreBootstrapped),
	string(apimgmtv3.ClusterConditionConnected),
	string(apimgmtv3.ClusterConditionAgentDeployed),
	string(apimgmtv3.ClusterConditionUpgraded),
}

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

func SummarizeCondition(cluster *apimgmtv3.Cluster) {
	if cluster == nil {
		return
	}

	clusterConditions := slices.Clone(cluster.Status.Conditions)
	if len(clusterConditions) == 0 {
		return
	}

	// build list of messages, making sure to truncate if over
	// for positive conditions that are false, we need to set false
	// for negative conditions that are true, we also need to set false
	// if all conditions are positive (good state), then we need to check our unknown conditions
	// if priority is important: surface all error messages. if no error messages, surface unknown (progressing).
	// if only green, mark cluster as Ready

	status := corev1.ConditionTrue
	for _, condition := range clusterConditions {
		if condition.Status == corev1.ConditionFalse {
			status = corev1.ConditionFalse
			break
		}
		if condition.Status == corev1.ConditionUnknown {
			status = corev1.ConditionUnknown
		}
	}

	var existingReady *apimgmtv3.ClusterCondition
	cons := []apimgmtv3.ClusterCondition{}
	for _, condition := range clusterConditions {
		if condition.Type == "Ready" {
			existingReady = &condition
			continue
		}
		if condition.Status == status {
			cons = append(cons, condition)
		}
	}

	order := make(map[string]int, len(Conditions))
	for rank, name := range Conditions {
		order[name] = rank
	}

	slices.SortFunc(cons, func(a, b apimgmtv3.ClusterCondition) int {
		r1, ok1 := order[string(a.Type)]
		r2, ok2 := order[string(b.Type)]

		if !ok1 {
			r1 = math.MaxInt
		}
		if !ok2 {
			r2 = math.MaxInt
		}

		if r1 != r2 {
			return cmp.Compare(r1, r2)
		}

		return cmp.Compare(string(a.Type), string(b.Type))
	})

	var messages []string
	if status != corev1.ConditionTrue {
		for _, condition := range cons {
			messages = append(messages, condition.Message)
		}
	}

	reason := "ClusterReady"
	if status == corev1.ConditionUnknown {
		reason = "ClusterProgressing"
	} else if status == corev1.ConditionFalse {
		reason = "ClusterNotReady"
	}

	ready := apimgmtv3.ClusterCondition{
		Type:    "Ready",
		Message: strings.Join(messages, "; "),
		Status:  status,
		Reason:  reason,
	}

	if existingReady != nil {
		if ready.Reason == existingReady.Reason &&
			ready.Message == existingReady.Message &&
			ready.Status == existingReady.Status {
			return
		}
	}

	apimgmtv3.ClusterConditionReady.SetStatus(cluster, string(ready.Status))
	apimgmtv3.ClusterConditionReady.Message(cluster, ready.Message)
	apimgmtv3.ClusterConditionReady.Reason(cluster, ready.Reason)
}
