package rkecerts

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	rkeCluster "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	v1 "k8s.io/api/core/v1"
)

func CleanCertificateBundle(certs map[string]pki.CertificatePKI) {
	for name := range certs {
		if strings.Contains(name, "token") || strings.Contains(name, "header") || strings.Contains(name, "admin") {
			delete(certs, name)
		}
	}
}

func GetCertExpiration(c string) (v32.CertExpiration, error) {
	date, err := GetCertExpirationDate(c)
	if err != nil {
		return v32.CertExpiration{}, err
	}
	return v32.CertExpiration{
		ExpirationDate: date.Format(time.RFC3339),
	}, nil
}

func GetCertExpirationDate(c string) (*time.Time, error) {
	certs, err := cert.ParseCertsPEM([]byte(c))
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, errors.New("no valid certs found")
	}
	return &certs[0].NotAfter, nil
}

func CertBundleFromConfig(cm *v1.ConfigMap) (map[string]pki.CertificatePKI, error) {
	if cm == nil {
		return nil, errors.New("full-cluster-state configmap not found")
	}
	rawCerts, ok := cm.Data[rkeCluster.FullStateConfigMapName]
	if !ok {
		return nil, errors.New("full-cluster-state configmap does not contain data")
	}
	rkeFullState := &rkeCluster.FullState{}
	err := json.Unmarshal([]byte(rawCerts), rkeFullState)
	if err != nil {
		return nil, err
	}
	return rkeFullState.CurrentState.CertificatesBundle, nil
}
