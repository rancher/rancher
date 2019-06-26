package cis

import (
	"fmt"
	"time"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterScanTypeCis = "cis"
)

func NewCISScan(cluster *v3.Cluster) *v3.ClusterScan {
	controller := true
	name := fmt.Sprintf("cis-%v", time.Now().UnixNano())
	return &v3.ClusterScan{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Name,
			OwnerReferences: []v1.OwnerReference{
				{
					Name:       cluster.Name,
					UID:        cluster.UID,
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Controller: &controller,
				},
			},
		},
		Spec: v3.ClusterScanSpec{
			ScanType:  ClusterScanTypeCis,
			ClusterID: cluster.Name,
		},
	}
}
