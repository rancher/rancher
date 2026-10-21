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
	string(apimgmtv3.ClusterConditionPending),
	string(apimgmtv3.ClusterConditionProvisioned),
	string(apimgmtv3.ClusterConditionWaiting),
	string(apimgmtv3.ClusterConditionPreBootstrapped),
	string(apimgmtv3.ClusterConditionAgentDeployed),
	string(apimgmtv3.ClusterConditionConnected),
	string(apimgmtv3.ClusterConditionProbeSucceeded),
	string(apimgmtv3.ClusterConditionControllersRunning),
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

// SummarizeReadyCondition aggregates the management cluster's important conditions into the
// [apimgmtv3.ClusterConditionReady] condition.
// If all conditions are "True", the "Ready" condition is marked "True" with the "ClusterReady" reason and no message.
// If any conditions are "False", their messages are aggregated and the "Ready" condition is marked "False" with the
// "ClusterNotReady" reason.
// If any conditions are "False", their messages are aggregated and the "Ready" condition is marked "Unknown" with the
// "ClusterProgressing" reason.
func SummarizeReadyCondition(cluster *apimgmtv3.Cluster) {
	if cluster == nil {
		return
	}

	if cluster.Status.ReadyReconciling {
		return
	}

	clusterConditions := slices.Clone(cluster.Status.Conditions)
	if len(clusterConditions) == 0 {
		return
	}

	order := make(map[string]int, len(Conditions))
	for rank, name := range Conditions {
		order[name] = rank
	}

	var existingReady *apimgmtv3.ClusterCondition

	status := corev1.ConditionTrue
	for _, condition := range clusterConditions {
		if condition.Type == "Ready" {
			existingReady = &condition
			continue
		}

		if _, ok := order[string(condition.Type)]; !ok {
			continue
		}

		if condition.Status == corev1.ConditionFalse {
			status = corev1.ConditionFalse
			break
		}
		if condition.Status == corev1.ConditionUnknown {
			status = corev1.ConditionUnknown
		}
	}

	cons := []apimgmtv3.ClusterCondition{}
	for _, condition := range clusterConditions {
		if condition.Type == "Ready" {
			continue
		}

		if _, ok := order[string(condition.Type)]; !ok {
			continue
		}

		if condition.Status == status {
			cons = append(cons, condition)
		}
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
			if condition.Message != "" {
				messages = append(messages, condition.Message)
			}
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

	if len(ready.Message) > 1024 {
		ready.Message = ready.Message[:1021] + "..."
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
