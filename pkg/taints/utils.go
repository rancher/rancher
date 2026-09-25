package taints

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// transientTaintPrefixes are the node lifecycle taints managed by kubelet, the
// node lifecycle controller or a cloud-controller-manager. They are added and
// removed automatically while a node comes up, goes down or loses its cloud
// provider initialization, so they must never be baked into a long lived pod
// spec: doing so makes the spec change every time a node blinks.
var transientTaintPrefixes = []string{
	// not-ready, unreachable, memory-pressure, disk-pressure, pid-pressure,
	// network-unavailable, unschedulable
	"node.kubernetes.io/",
	// uninitialized, shutdown
	"node.cloudprovider.kubernetes.io/",
}

// IsTransient returns true when the taint is a node lifecycle taint that a
// controller adds and removes on its own, rather than a durable taint set by an
// administrator or by the cluster's role configuration.
func IsTransient(taint v1.Taint) bool {
	for _, prefix := range transientTaintPrefixes {
		if strings.HasPrefix(taint.Key, prefix) {
			return true
		}
	}
	return false
}

// Sort orders taints in place by key, then effect, then value.
func Sort(taints []v1.Taint) {
	sort.Slice(taints, func(i, j int) bool {
		if taints[i].Key != taints[j].Key {
			return taints[i].Key < taints[j].Key
		}
		if taints[i].Effect != taints[j].Effect {
			return taints[i].Effect < taints[j].Effect
		}
		return taints[i].Value < taints[j].Value
	})
}

func GetTaintsString(taint v1.Taint) string {
	return fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
}

func GetKeyEffectString(taint v1.Taint) string {
	return fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
}

func GetTaintFromString(taintStr string) *v1.Taint {
	taintStruct := strings.Split(taintStr, "=")
	if len(taintStruct) != 2 {
		logrus.Warnf("taint string %s is not validated", taintStr)
		return nil
	}
	tmp := strings.Split(taintStruct[1], ":")
	if len(tmp) != 2 {
		logrus.Warnf("taint string %s is not validated", taintStr)
		return nil
	}
	key := taintStruct[0]
	value := tmp[0]
	effect := v1.TaintEffect(tmp[1])
	return &v1.Taint{
		Key:    key,
		Value:  value,
		Effect: effect,
	}
}

func GetTaintSet(taints []v1.Taint) map[string]int {
	rtn := map[string]int{}
	for i, taint := range taints {
		rtn[GetTaintsString(taint)] = i
	}
	return rtn
}

func GetKeyEffectTaintSet(taints []v1.Taint) map[string]int {
	rtn := map[string]int{}
	for i, taint := range taints {
		rtn[GetKeyEffectString(taint)] = i
	}
	return rtn
}

func GetToDiffTaints(current, desired []v1.Taint) (toAdd map[int]v1.Taint, toDel map[int]v1.Taint) {
	toAdd, toDel = map[int]v1.Taint{}, map[int]v1.Taint{}
	currentSet := GetTaintSet(current)
	desiredSet := GetTaintSet(desired)
	for k, index := range currentSet {
		if _, ok := desiredSet[k]; !ok {
			toDel[index] = current[index]
		}
	}
	for k, index := range desiredSet {
		if _, ok := currentSet[k]; !ok {
			toAdd[index] = desired[index]
		}
	}
	return toAdd, toDel
}

func GetTaintsFromStrings(sources []string) []v1.Taint {
	var rtn []v1.Taint
	for _, source := range sources {
		taint := GetTaintFromString(source)
		if taint == nil {
			continue
		}
		rtn = append(rtn, *taint)
	}
	return rtn
}

// MergeTaints will override t1 taint by t2 with same key and effect
func MergeTaints(t1 []v1.Taint, t2 []v1.Taint) []v1.Taint {
	set1 := GetKeyEffectTaintSet(t1)
	set2 := GetKeyEffectTaintSet(t2)
	rtn := t2
	for key, i := range set1 {
		if j, ok := set2[key]; ok {
			logrus.Infof("overriding taint %s with %s", GetTaintsString(t1[i]), GetTaintsString(t2[j]))
			continue
		}
		rtn = append(rtn, t1[i])
	}
	return rtn
}
