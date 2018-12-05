package globaldns

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	k8srbacV1 "k8s.io/api/rbac/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	extv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
)

const (
	GlobaldnsProviderLauncher = "mgmt-global-dns-provider-launcher"
)

type ProviderLauncher struct {
	GlobalDNSproviders      v3.GlobalDNSProviderInterface
	GlobalDNSproviderLister v3.GlobalDNSProviderLister
	Deployments             extv1beta1.DeploymentInterface
	ServiceAccounts         corev1.ServiceAccountInterface
	ClusterRoles            rbacv1.ClusterRoleInterface
	ClusterRoleBindings     rbacv1.ClusterRoleBindingInterface
}

func newGlobalDNSProviderLauncher(ctx context.Context, mgmt *config.ManagementContext) *ProviderLauncher {
	n := &ProviderLauncher{
		GlobalDNSproviders:      mgmt.Management.GlobalDNSProviders(namespace.GlobalNamespace),
		GlobalDNSproviderLister: mgmt.Management.GlobalDNSProviders(namespace.GlobalNamespace).Controller().Lister(),
		Deployments:             mgmt.K8sClient.ExtensionsV1beta1().Deployments(namespace.GlobalNamespace),
		ServiceAccounts:         mgmt.K8sClient.CoreV1().ServiceAccounts(namespace.GlobalNamespace),
		ClusterRoles:            mgmt.K8sClient.RbacV1beta1().ClusterRoles(),
		ClusterRoleBindings:     mgmt.K8sClient.RbacV1beta1().ClusterRoleBindings(),
	}
	return n
}

//sync is called periodically and on real updates
func (n *ProviderLauncher) sync(key string, obj *v3.GlobalDNSProvider) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	//check if provider already running for this GlobalDNSProvider.
	if n.isProviderAlreadyRunning(obj) {
		logrus.Infof("GlobaldnsProviderLauncher: Found a running external-dns deployment for this provider %v , skip creating a new one", obj.Name)
		return nil, nil
	}

	//create svcAcct and rbac entities if not created already
	err := n.createExternalDNSServiceAccount()
	if err != nil {
		return nil, err
	}

	err = n.createExternalDNSClusterRole()
	if err != nil {
		return nil, err
	}

	err = n.createExternalDNSClusterRoleBinding()
	if err != nil {
		return nil, err
	}

	//handle external-dns deployment
	if obj.Spec.Route53ProviderConfig != nil {
		return n.handleRoute53Provider(obj)
	}

	return nil, nil
}

func (n *ProviderLauncher) handleRoute53Provider(obj *v3.GlobalDNSProvider) (runtime.Object, error) {
	//create external-dns route53 provider
	data := map[string]interface{}{
		"awsAccessKey":   obj.Spec.Route53ProviderConfig.AccessKey,
		"awsSecretKey":   obj.Spec.Route53ProviderConfig.SecretKey,
		"route53Domain":  obj.Spec.Route53ProviderConfig.RootDomain,
		"deploymentName": obj.Name,
	}
	route53Template := template.Must(template.New("route53_template").Parse(Route53DeploymentTemplate))
	var output bytes.Buffer
	if err := route53Template.Execute(&output, data); err != nil {
		return nil, fmt.Errorf("GlobaldnsProviderLauncher: Error parsing the external-dns/route53 deployment template: %v", err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	deployObj, _, err := decode(output.Bytes(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("GlobaldnsProviderLauncher: Error decoding  external-dns/route53 deployment template to Kubernetes Object: %v", err)
	}

	deployment := deployObj.(*v1beta1.Deployment)
	//set ownerRef to the dnsProvider CR
	controller := true
	ownerRef := []metav1.OwnerReference{{
		Name:       obj.Name,
		APIVersion: "v3",
		UID:        obj.UID,
		Kind:       obj.Kind,
		Controller: &controller,
	}}
	deployment.ObjectMeta.OwnerReferences = ownerRef

	deploymentCreated, err := n.Deployments.Create(deployment)
	if err != nil {
		return nil, fmt.Errorf("GlobaldnsProviderLauncher: Error creating external-dns deployment for Route53 provider: %v ", err)
	}
	logrus.Infof("GlobaldnsProviderLauncher: Created Route53 external-dns provider deployment %v", deploymentCreated.Name)

	return nil, nil
}

func (n *ProviderLauncher) isProviderAlreadyRunning(obj *v3.GlobalDNSProvider) bool {
	existingDep, err := n.Deployments.Get("route53", metav1.GetOptions{})

	if (err != nil && k8serrors.IsNotFound(err)) || existingDep == nil {
		return false
	} else if err != nil && !k8serrors.IsNotFound(err) {
		logrus.Errorf("GlobaldnsProviderLauncher: Error listing external-dns %v Deployment %v", obj.Name, err)
		return false
	}

	return true
}

func (n *ProviderLauncher) createExternalDNSServiceAccount() error {
	decode := scheme.Codecs.UniversalDeserializer().Decode

	//create svcAcct and bindings
	_, err := n.ServiceAccounts.Get("external-dns", metav1.GetOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("GlobaldnsProviderLauncher: Error listing external-dns ServiceAccount %v", err)
	} else if k8serrors.IsNotFound(err) {
		svcAcctObj, _, err := decode([]byte(ExternalDNSServiceAcct), nil, nil)
		if err != nil {
			return fmt.Errorf("GlobaldnsProviderLauncher: Error decoding external-dns ServiceAccount to Kubernetes Object: %v", err)
		}
		svcAcct := svcAcctObj.(*v1.ServiceAccount)
		_, err = n.ServiceAccounts.Create(svcAcct)
		if err != nil {
			return fmt.Errorf("GlobaldnsProviderLauncher: Error creating external-dns ServiceAccount: %v ", err)
		}
	}
	return nil
}

func (n *ProviderLauncher) createExternalDNSClusterRole() error {
	decode := scheme.Codecs.UniversalDeserializer().Decode

	_, err := n.ClusterRoles.Get("external-dns", metav1.GetOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("GlobaldnsProviderLauncher: Error listing external-dns ClusterRole %v", err)
	} else if k8serrors.IsNotFound(err) {
		crObj, _, err := decode([]byte(ExternalDNSClusterRole), nil, nil)
		if err != nil {
			return fmt.Errorf("GlobaldnsProviderLauncher: Error decoding external-dns ClusterRole to Kubernetes Object: %v", err)
		}
		cr := crObj.(*k8srbacV1.ClusterRole)
		_, err = n.ClusterRoles.Create(cr)
		if err != nil {
			return fmt.Errorf("GlobaldnsProviderLauncher: Error creating external-dns ClusterRole: %v ", err)
		}
	}
	return nil
}

func (n *ProviderLauncher) createExternalDNSClusterRoleBinding() error {
	decode := scheme.Codecs.UniversalDeserializer().Decode

	_, err := n.ClusterRoleBindings.Get("external-dns-viewer", metav1.GetOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("GlobaldnsProviderLauncher: Error listing external-dns-viewer ClusterRoleBinding %v", err)
	} else if k8serrors.IsNotFound(err) {
		crBindingObj, _, err := decode([]byte(ExternalDNSClusterRoleBinding), nil, nil)
		if err != nil {
			return fmt.Errorf("GlobaldnsProviderLauncher; Error decoding external-dns-viewer ClusterRoleBinding to Kubernetes Object: %v", err)
		}
		crBinding := crBindingObj.(*k8srbacV1.ClusterRoleBinding)
		_, err = n.ClusterRoleBindings.Create(crBinding)
		if err != nil {
			return fmt.Errorf("GlobaldnsProviderLauncher: Error creating external-dns-viewer ClusterRoleBinding: %v ", err)
		}
	}

	return nil
}
