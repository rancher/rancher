package rkecerts

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	rkeCluster "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/pki"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v1 "k8s.io/api/core/v1"
)

var exampleCert = "-----BEGIN CERTIFICATE-----\nMIICpDCCAYwCCQCPcjBFQaNrJjANBgkqhkiG9w0BAQsFADAUMRIwEAYDVQQDDAls\nb2NhbGhvc3QwHhcNMjAwMjI1MTYxMDAxWhcNMjEwMjI0MTYxMDAxWjAUMRIwEAYD\nVQQDDAlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDj\nPcg9ksAr44psuf0SHwFQC6GceTNHggKZDAqADlhFCZ9P5kb+JUJJ86u08LLDSBDF\nl0EsQSPbmCJmr7mnUf7byBBC8/5pTiZXIHM7VUhjL5Ooq8D9xbylTah8fMmcQbdc\nv8RffbHIpQ7oEHrpfEdv8FeIdpQEdiCVzZBV6LX/Cw5YkJvAx4P/E7Kf2c99YGeP\nmRxI94vThrd3mtFCzyyXgrW1wUtbBipFC/y0vpVhCceDAWThQeSF6ZwjxXOjUqvC\netME1jwnnn7al2GmbcfhY8sx73EQwbQI+Kn5sul+oixRHFL4XFZIWCYe2YXJ2dpC\n7SxF+844YT4fEyItEG83AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAErzQ7eS1rB8\nOx/we9yC3X3z+5V0cH91tyXvGrVYJuWN+kdv/bQP0Gu+Fvk+82jHcoQS8hRn77t+\noyfph/lk8WicsllVud06Z7K16akxzBtSUahkw38UuVxuQ8U5ZuH5JkyZVcRyq210\nR3sn5U9gxFZ3zISfWhZI8EXU/K7IB03Bv3HG0uZwRpI8w5O6jC9vD2hoFHPpqlTQ\nfVpQjALKswZWdN5Dm7YP9JpUjWrl6lFmkE2cpj+F0cZHIgWupsBgVT7WwRUGgZPN\nstf+yTf2og2fVciZuopzfMhk545Zwye7CUseOP6YOeWKnm/UbR314fKX7Rum1saM\nbVQypIiA8ds=\n-----END CERTIFICATE-----\n"

func TestGetCertExpirationDate(t *testing.T) {
	tests := []struct {
		name    string
		cert    string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "return correct date",
			cert:    exampleCert,
			want:    time.Date(2021, 02, 24, 16, 10, 01, 0, time.UTC),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCertExpirationDate(tt.cert)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCertExpirationDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.want.Equal(*got) {
				t.Errorf("GetCertExpirationDate() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCertExpiration(t *testing.T) {
	tests := []struct {
		name    string
		cert    string
		want    v3.CertExpiration
		wantErr bool
	}{
		{
			name: "valid cert",
			cert: exampleCert,
			want: v3.CertExpiration{
				ExpirationDate: "2021-02-24T16:10:01Z",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCertExpiration(tt.cert)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCertExpiration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCertExpiration() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCertBundleFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		configMap v1.ConfigMap
		want      map[string]pki.CertificatePKI
		wantErr   bool
	}{
		{
			name: "valid configmap",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					rkeCluster.FullStateConfigMapName: fmt.Sprintf("{\"currentState\":{\"certificatesBundle\":{\"kube-admin\":{\"certificatePEM\":\"%s\"}}}}", strings.Replace(exampleCert, "\n", `\n`, -1)),
				},
			},
			want: map[string]pki.CertificatePKI{
				"kube-admin": pki.CertificatePKI{
					CertificatePEM: exampleCert,
				},
			},
			wantErr: false,
		},
		{
			name:      "invalid configmap",
			configMap: v1.ConfigMap{},
			want:      nil,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CertBundleFromConfig(&tt.configMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("CertBundleFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CertBundleFromConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
