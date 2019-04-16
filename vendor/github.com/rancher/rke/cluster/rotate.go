package cluster

import (
	"context"
	"fmt"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
)

func RotateRKECertificates(ctx context.Context, kubeCluster, currentCluster *Cluster) error {
	if kubeCluster.Authentication.Strategy == X509AuthenticationProvider {
		var err error
		backupPlane := fmt.Sprintf("%s,%s", EtcdPlane, ControlPlane)
		backupHosts := hosts.GetUniqueHostList(kubeCluster.EtcdHosts, kubeCluster.ControlPlaneHosts, nil)
		if currentCluster != nil {
			kubeCluster.Certificates = currentCluster.Certificates
			// this is the case of handling upgrades for API server aggregation layer ca cert and API server proxy client key and cert
			if kubeCluster.Certificates[pki.RequestHeaderCACertName].Certificate == nil {

				kubeCluster.Certificates, err = regenerateAPIAggregationCerts(kubeCluster, kubeCluster.Certificates)
				if err != nil {
					return fmt.Errorf("Failed to regenerate Aggregation layer certificates %v", err)
				}
			}
		} else {
			log.Infof(ctx, "[certificates] Attempting to recover certificates from backup on [%s] hosts", backupPlane)

			kubeCluster.Certificates, err = fetchBackupCertificates(ctx, backupHosts, kubeCluster)
			if err != nil {
				return err
			}
			if kubeCluster.Certificates != nil {
				log.Infof(ctx, "[certificates] Certificate backup found on [%s] hosts", backupPlane)

				// make sure I have all the etcd certs, We need handle dialer failure for etcd nodes https://github.com/rancher/rancher/issues/12898
				for _, host := range kubeCluster.EtcdHosts {
					certName := pki.GetEtcdCrtName(host.InternalAddress)
					if kubeCluster.Certificates[certName].Certificate == nil {
						if kubeCluster.Certificates, err = pki.RegenerateEtcdCertificate(ctx,
							kubeCluster.Certificates,
							host,
							kubeCluster.EtcdHosts,
							kubeCluster.ClusterDomain,
							kubeCluster.KubernetesServiceIP); err != nil {
							return err
						}
					}
				}
				// this is the case of adding controlplane node on empty cluster with only etcd nodes
				if kubeCluster.Certificates[pki.KubeAdminCertName].Config == "" && len(kubeCluster.ControlPlaneHosts) > 0 {
					if err := rebuildLocalAdminConfig(ctx, kubeCluster); err != nil {
						return err
					}
					kubeCluster.Certificates, err = regenerateAPICertificate(kubeCluster, kubeCluster.Certificates)
					if err != nil {
						return fmt.Errorf("Failed to regenerate KubeAPI certificate %v", err)
					}
				}
				// this is the case of handling upgrades for API server aggregation layer ca cert and API server proxy client key and cert
				if kubeCluster.Certificates[pki.RequestHeaderCACertName].Certificate == nil {

					kubeCluster.Certificates, err = regenerateAPIAggregationCerts(kubeCluster, kubeCluster.Certificates)
					if err != nil {
						return fmt.Errorf("Failed to regenerate Aggregation layer certificates %v", err)
					}
				}
			}
		}
		log.Infof(ctx, "[certificates] Rotating RKE certificates")

		kubeCluster.Certificates, err = pki.RotateRKECerts(ctx, kubeCluster.Certificates, kubeCluster.RancherKubernetesEngineConfig, kubeCluster.LocalKubeConfigPath, "")
		if err != nil {
			return fmt.Errorf("Failed to generate Kubernetes certificates: %v", err)
		}
		log.Infof(ctx, "[certificates] Temporarily saving certs to [%s] hosts", backupPlane)
		if err := deployBackupCertificates(ctx, backupHosts, kubeCluster, true); err != nil {
			return err
		}
		log.Infof(ctx, "[certificates] Saved certs to [%s] hosts", backupPlane)
	}
	return nil
}
