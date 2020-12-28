package deployer

import (
	"testing"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/managementuser/logging/config"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"

	"github.com/stretchr/testify/assert"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestIsDeploySuccess(t *testing.T) {

	fakeLabels := map[string]string{"app": "fake"}
	fluentdLabels := map[string]string{"app": "fluentd"}

	err := testIsDeploySuccess(fluentdLabels, k8scorev1.PodRunning, fakeLabels, k8scorev1.PodRunning)
	assert.Nil(t, err)

	err = testIsDeploySuccess(fluentdLabels, k8scorev1.PodRunning, fakeLabels, k8scorev1.PodPending)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "timeout")

}

func testIsDeploySuccess(expectLabels map[string]string, expectedPhase k8scorev1.PodPhase, actualLabels map[string]string, actualPhase k8scorev1.PodPhase) error {
	appDeployer := AppDeployer{
		PodLister: &fakes.PodListerMock{
			ListFunc: getPods(actualLabels, actualPhase),
		},
	}

	return appDeployer.isDeploySuccess(loggingconfig.LoggingNamespace, expectLabels)
}

func getPods(podLabels map[string]string, phase k8scorev1.PodPhase) func(namespace string, selector labels.Selector) ([]*k8scorev1.Pod, error) {
	return func(namespace string, selector labels.Selector) ([]*k8scorev1.Pod, error) {
		return []*k8scorev1.Pod{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels:    podLabels,
					Namespace: loggingconfig.LoggingNamespace,
				},
				Status: k8scorev1.PodStatus{
					Phase: phase,
				},
			},
		}, nil
	}
}
