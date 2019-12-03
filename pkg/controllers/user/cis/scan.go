package cis

import (
	"fmt"
	"time"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewCisScan(cluster *v3.Cluster, cisScanConfig *v3.CisScanConfig) *v3.ClusterScan {
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
			ScanType:  v3.ClusterScanTypeCis,
			ClusterID: cluster.Name,
			ScanConfig: v3.ClusterScanConfig{
				CisScanConfig: cisScanConfig,
			},
		},
	}
}
