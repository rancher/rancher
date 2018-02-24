package networkpolicy

import (
	"fmt"
	"reflect"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/condition"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

type mgr struct {
	pnpLister v3.ProjectNetworkPolicyLister
	pnpClient *clientbase.ObjectClient
	nsLister  typescorev1.NamespaceLister
	nsClient  *clientbase.ObjectClient
	K8sClient kubernetes.Interface
}

type projectSyncer struct {
	pnpLister  v3.ProjectNetworkPolicyLister
	pnpClient  *clientbase.ObjectClient
	projClient *clientbase.ObjectClient
}

// Sync is responsible for creating a default ProjectNetworkPolicy for
// every project created. There is no need to worry about clean up, as
// this pnp object is tied to the namespace of the project, it's deleted
// automatically.
func (ps *projectSyncer) Sync(key string, p *v3.Project) error {
	if p == nil {
		return nil
	}

	pcopy := p.DeepCopyObject()

	pcopy, err := ps.createDefaultNetworkPolicy(pcopy)
	if err != nil {
		return err
	}

	// update if it has changed
	if pcopy != nil && !reflect.DeepEqual(p, pcopy) {
		_, err = ps.projClient.Update(p.Name, pcopy)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ps *projectSyncer) createDefaultNetworkPolicy(p runtime.Object) (runtime.Object, error) {
	return v3.DefaultNetworkPolicyCreated.Do(p, func() (runtime.Object, error) {
		o, err := meta.Accessor(p)
		if err != nil {
			return p, condition.Error("MissingMetadata", err)
		}

		projectName := o.GetName()
		defaultPolicyName := "pnp-" + projectName
		existingPolicies, err := ps.pnpLister.List(defaultPolicyName, labels.Everything())
		if err != nil {
			logrus.Errorf("error fetching existing project network policy: %v", err)
			return p, err
		}
		if len(existingPolicies) == 0 {
			pnpDesc := fmt.Sprintf("Default network policy for project %v", projectName)
			_, err = ps.pnpClient.Create(&v3.ProjectNetworkPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      defaultPolicyName,
					Namespace: projectName,
				},
				Spec: v3.ProjectNetworkPolicySpec{
					Description: pnpDesc,
					ProjectName: o.GetNamespace() + ":" + projectName,
				},
			})
			if err == nil {
				logrus.Infof("Successfully created default network policy for project: %v", projectName)
			}
		}

		return p, nil
	})
}
