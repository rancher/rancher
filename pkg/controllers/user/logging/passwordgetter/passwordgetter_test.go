package passwordgetter

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	fake1 "github.com/rancher/rancher/pkg/types/apis/core/v1/fakes"
	mgmtv3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
)

const (
	passwordSecretValue = "password"
	secretName          = "secret-uuid-xxx"
	secretNamespace     = "cattle-global-data"
	userName            = "user1"
	esEndpoint          = "https://localhost:9200"
	fluentdEndpoint     = "https://localhost:24224"
)

var (
	passwordWrapValue = secretNamespace + ":" + secretName
)

var tests = []struct {
	in  mgmtv3.LoggingTargets
	out mgmtv3.LoggingTargets
}{
	{in: elasticTarget(passwordWrapValue), out: elasticTarget(passwordSecretValue)},
	{in: fluentdTarget(passwordWrapValue), out: fluentdTarget(passwordSecretValue)},
}

var (
	passwordSecret = &v1.Secret{
		Data: map[string][]byte{
			secretName: []byte(passwordSecretValue),
		},
	}

	secretInterface = fake1.SecretInterfaceMock{
		GetNamespacedFunc: func(namespace string, name string, opts metav1.GetOptions) (*v1.Secret, error) {
			return passwordSecret, nil
		},
		ControllerFunc: func() corev1.SecretController {
			return &fake1.SecretControllerMock{
				ListerFunc: func() corev1.SecretLister {
					return &fake1.SecretListerMock{
						GetFunc: func(namespace string, name string) (*v1.Secret, error) {
							return passwordSecret, nil
						},
					}
				},
			}
		},
	}
)

func TestReadFromSecret(t *testing.T) {
	passwordGatter := NewPasswordGetter(&secretInterface)
	for _, tt := range tests {
		if err := passwordGatter.GetPasswordFromSecret(&tt.in); err != nil {
			t.Error(err)
			continue
		}

		if !reflect.DeepEqual(tt.in, tt.out) {
			t.Error("output not equal to expected value")
		}
	}
}

func elasticTarget(password string) mgmtv3.LoggingTargets {
	return mgmtv3.LoggingTargets{
		ElasticsearchConfig: &mgmtv3.ElasticsearchConfig{
			Endpoint:     esEndpoint,
			AuthUserName: userName,
			AuthPassword: password,
		},
	}
}

func fluentdTarget(password string) mgmtv3.LoggingTargets {
	return mgmtv3.LoggingTargets{
		FluentForwarderConfig: &mgmtv3.FluentForwarderConfig{
			FluentServers: []mgmtv3.FluentServer{
				{
					Endpoint: fluentdEndpoint,
					Username: userName,
					Password: password,
				},
			},
		},
	}
}
