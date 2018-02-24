package networkpolicy

import (
	"fmt"
	"reflect"

	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type netpolMgr struct {
	nsLister  typescorev1.NamespaceLister
	k8sClient kubernetes.Interface
}

func (npmgr *netpolMgr) programNetworkPolicy(projectID string) error {
	logrus.Debugf("programNetworkPolicy: projectID=%v", projectID)
	// Get namespaces belonging to project
	set := labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: projectID})
	namespaces, err := npmgr.nsLister.List("", set.AsSelector())
	if err != nil {
		logrus.Errorf("programNetworkPolicy err=%v", err)
		return fmt.Errorf("couldn't list namespaces with projectID %v err=%v", projectID, err)
	}
	logrus.Debugf("namespaces=%+v", namespaces)

	for _, aNS := range namespaces {
		policyName := "np-default"
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: v1.ObjectMeta{
				Name:      policyName,
				Namespace: aNS.Name,
				Labels:    labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: projectID}),
			},
			Spec: networkingv1.NetworkPolicySpec{
				// An empty PodSelector selects all pods in this Namespace.
				PodSelector: v1.LabelSelector{},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkingv1.NetworkPolicyIngressRule{
						From: []networkingv1.NetworkPolicyPeer{
							networkingv1.NetworkPolicyPeer{
								NamespaceSelector: &v1.LabelSelector{
									MatchLabels: map[string]string{nslabels.ProjectIDFieldLabel: projectID},
								},
							},
						},
					},
				},
			},
		}

		existing, err := npmgr.k8sClient.NetworkingV1().NetworkPolicies(aNS.Name).Get(policyName, v1.GetOptions{})
		logrus.Debugf("programNetworkPolicy existing=%+v, err=%v", existing, err)
		if err != nil {
			if kerrors.IsNotFound(err) {
				logrus.Debugf("about to create np=%+v", *np)
				_, err = npmgr.k8sClient.NetworkingV1().NetworkPolicies(aNS.Name).Create(np)
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
				_, err = npmgr.k8sClient.NetworkingV1().NetworkPolicies(aNS.Name).Update(np)
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
