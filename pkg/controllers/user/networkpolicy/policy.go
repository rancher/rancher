package networkpolicy

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rancher/norman/clientbase"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	projectIDField = "field.cattle.io/projectId"
)

type mgr struct {
	pnpLister v3.ProjectNetworkPolicyLister
	pnpClient *clientbase.ObjectClient
	nsLister  typescorev1.NamespaceLister
	nsClient  *clientbase.ObjectClient
	K8sClient kubernetes.Interface
}

type projectNetworkPolicyLifecycle struct {
	mgr *mgr
}

type projectSyncer struct {
	mgr *mgr
}

type namespaceLifecycle struct {
	mgr *mgr
}

func Register(cluster *config.UserContext) {
	logrus.Infof("Registering project network policy")

	mgr := &mgr{
		cluster.Management.Management.ProjectNetworkPolicies("").Controller().Lister(),
		cluster.Management.Management.ProjectNetworkPolicies("").ObjectClient(),
		cluster.Core.Namespaces("").Controller().Lister(),
		cluster.Core.Namespaces("").ObjectClient(),
		cluster.K8sClient,
	}

	ps := &projectSyncer{mgr: mgr}
	nslc := &namespaceLifecycle{mgr: mgr}
	pnplc := &projectNetworkPolicyLifecycle{mgr: mgr}

	cluster.Management.Management.Projects("").Controller().AddHandler("projectSyncer", ps.Sync)
	cluster.Management.Management.ProjectNetworkPolicies("").AddLifecycle("projectNetworkPolicyLifecycle", pnplc)
	cluster.Core.Namespaces("").AddLifecycle("namespaceLifecycle", nslc)
}

func (pnplc *projectNetworkPolicyLifecycle) Create(pnp *v3.ProjectNetworkPolicy) (*v3.ProjectNetworkPolicy, error) {
	return pnp, nil
}

func (pnplc *projectNetworkPolicyLifecycle) Updated(pnp *v3.ProjectNetworkPolicy) (*v3.ProjectNetworkPolicy, error) {
	logrus.Infof("pnplc Updated pnp=%+v", pnp)

	projectID, ok := pnp.Labels[projectIDField]
	if !ok {
		return pnp, fmt.Errorf("couldn't find projectID for pnp=%+v", *pnp)
	}
	logrus.Debugf("pnplc Updated projectID: %v", projectID)

	err := programNetworkPolicy(pnplc.mgr, projectID)
	return pnp, err
}

func (pnplc *projectNetworkPolicyLifecycle) Remove(pnp *v3.ProjectNetworkPolicy) (*v3.ProjectNetworkPolicy, error) {
	logrus.Infof("pnplc Remove pnp=%+v", pnp)

	projectID, ok := pnp.Labels[projectIDField]
	if !ok {
		return pnp, fmt.Errorf("couldn't find projectIDField for pnp=%+v", *pnp)
	}
	logrus.Debugf("pnplc Updated projectID: %v", projectID)

	set := labels.Set(map[string]string{projectIDField: projectID})

	err := pnplc.mgr.K8sClient.NetworkingV1().NetworkPolicies("").DeleteCollection(nil, v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		logrus.Errorf("error deleting network policies for projectID=%v", projectID)
		return pnp, err
	}
	return pnp, nil
}

func (ps *projectSyncer) Sync(key string, p *v3.Project) error {
	if p == nil {
		return nil
	}
	return ps.mgr.createDefaultNetworkPolicy(p)
}

func (mgr *mgr) createDefaultNetworkPolicy(p *v3.Project) error {
	if p == nil {
		return nil
	}
	existingPolicies, err := mgr.pnpLister.List(p.Name, labels.Everything())
	if err != nil {
		logrus.Errorf("error fetching existing project network policy: %v", err)
		return err
	}
	if len(existingPolicies) > 0 {
		logrus.Debugf("default network policy for project=%v already exists: %+v", p.Name, existingPolicies)
		return nil
	}

	pnpDesc := fmt.Sprintf("Default network policy for project %v", p.Name)
	logrus.Infof("creating default network policy for project %v", p.Name)
	_, err = mgr.pnpClient.Create(&v3.ProjectNetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("projectnetworkpolicy-%v", p.Name),
			Namespace: p.Name,
			Annotations: map[string]string{
				projectIDField: p.Namespace + ":" + p.Name,
			},
			Labels: labels.Set(map[string]string{projectIDField: p.Name}),
		},
		Spec: v3.ProjectNetworkPolicySpec{
			Description: pnpDesc,
			ProjectName: p.Namespace + ":" + p.Name,
		},
	})
	if err == nil {
		logrus.Infof("Successfully created default network policy for project: %v", p.Name)
	}
	return err
}

func (nslc *namespaceLifecycle) Create(ns *corev1.Namespace) (*corev1.Namespace, error) {
	return ns, nil
}

func (nslc *namespaceLifecycle) Updated(ns *corev1.Namespace) (*corev1.Namespace, error) {
	logrus.Infof("nslc Updated: %+v", *ns)

	metaAccessor, err := meta.Accessor(ns)
	if err != nil {
		return ns, nil
	}

	field, ok := metaAccessor.GetAnnotations()[projectIDField]
	if !ok {
		return ns, nil
	}

	splits := strings.Split(field, ":")
	if len(splits) != 2 {
		return ns, nil
	}
	projectID := splits[1]
	logrus.Debugf("nslc Updated: projectID=%v", projectID)
	return ns, programNetworkPolicy(nslc.mgr, projectID)
}

func (nslc *namespaceLifecycle) Remove(ns *corev1.Namespace) (*corev1.Namespace, error) {
	return ns, nil
}

func programNetworkPolicy(mgr *mgr, projectID string) error {
	logrus.Debugf("programNetworkPolicy: projectID=%v", projectID)
	// Get namespaces belonging to project
	set := labels.Set(map[string]string{projectIDField: projectID})
	//namespaces, err := mgr.nsClient.List(v1.ListOptions{LabelSelector: set.String()})
	namespaces, err := mgr.nsLister.List("", set.AsSelector())
	if err != nil {
		logrus.Errorf("programNetworkPolicy err=%v", err)
		return fmt.Errorf("couldn't list namespaces with projectID %v err=%v", projectID, err)
	}
	logrus.Debugf("namespaces=%+v", namespaces)

	for _, aNS := range namespaces {
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: v1.ObjectMeta{
				Name:      "default", // TODO: Fix this
				Namespace: aNS.Name,
				Labels:    labels.Set(map[string]string{projectIDField: projectID}),
			},
			Spec: networkingv1.NetworkPolicySpec{
				// An empty PodSelector selects all pods in this Namespace.
				PodSelector: v1.LabelSelector{},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkingv1.NetworkPolicyIngressRule{
						From: []networkingv1.NetworkPolicyPeer{
							networkingv1.NetworkPolicyPeer{
								NamespaceSelector: &v1.LabelSelector{
									MatchLabels: map[string]string{projectIDField: projectID},
								},
							},
						},
					},
				},
			},
		}

		existing, err := mgr.K8sClient.NetworkingV1().NetworkPolicies(aNS.Name).Get("default", v1.GetOptions{})
		logrus.Debugf("programNetworkPolicy existing=%+v, err=%v", existing, err)
		if err != nil {
			if kerrors.IsNotFound(err) {
				logrus.Debugf("about to create np=%+v", *np)
				_, err = mgr.K8sClient.NetworkingV1().NetworkPolicies(aNS.Name).Create(np)
				if err != nil {
					logrus.Errorf("programNetworkPolicy: error creating network policy err=%v", err)
					return err
				}

			} else {
				logrus.Errorf("programNetworkPolicy: got unexpected error while getting network policy=%v", err)
			}
		} else {
			logrus.Debugf("programNetworkPolicy: existing=%+v", existing)
			if !reflect.DeepEqual(existing, np) {
				logrus.Debugf("about to update np=%+v", *np)
				_, err = mgr.K8sClient.NetworkingV1().NetworkPolicies(aNS.Name).Update(np)
				if err != nil {
					logrus.Errorf("programNetworkPolicy: error updating network policy err=%v", err)
					return err
				}
			} else {
				logrus.Debugf("no need to update np=%+v", *np)
			}
		}
	}
	return nil
}
