/* Following objects had Roles and RoleBindings with incorrect OwnerRefs prior to 2.4.6:
Cloud Credential (Secret representing a cloud credential)
Node Template
Cluster Template
Cluster Template Revision
Global DNS Entry
Global DNS Provider
Multi Cluster App
Multi Cluster App Revision

this controller updates the ownerReferences on these roles and rolebindings to use the correct Kind/APIVersion fields
*/

package rbac

import (
	"context"

	typesv1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	ownerRefUpdated = "auth.management.cattle.io/owner-ref-updated"
)

type ownerRefCleaner struct {
	roles        v1.RoleInterface
	roleBindings v1.RoleBindingInterface
}

func Register(ctx context.Context, m *config.ManagementContext) {
	o := ownerRefCleaner{
		roles:        m.RBAC.Roles(""),
		roleBindings: m.RBAC.RoleBindings(""),
	}
	o.roles.AddHandler(ctx, "legacy-role-ownerref-cleaner", o.roleSync)
	o.roleBindings.AddHandler(ctx, "legacy-rolebinding-ownerref-cleaner", o.roleBindingSync)
}

// roleSync updates roles for these resources that have incorrect ownerRefs
func (o *ownerRefCleaner) roleSync(key string, role *v1.Role) (runtime.Object, error) {
	if role == nil || role.DeletionTimestamp != nil {
		return nil, nil
	}
	if val, ok := role.Labels[ownerRefUpdated]; ok && val == "true" {
		return nil, nil
	}
	// All these resources are either created in cattle-global-data ns, or in case of node templates in cattle-global-nt ns
	if role.Namespace != namespaces.GlobalNamespace && role.Namespace != namespaces.NodeTemplateGlobalNamespace {
		return nil, nil
	}
	if len(role.OwnerReferences) == 0 {
		return nil, nil
	}

	needsUpdate := correctOwnerRefs(&role.OwnerReferences)
	if !needsUpdate {
		return nil, nil
	}
	logrus.Infof("Updating ownerReferences of Role %v/%v", role.Namespace, role.Name)

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		roleToUpdate, updateErr := o.roles.GetNamespaced(role.Namespace, role.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		roleToUpdate.OwnerReferences = role.OwnerReferences
		if roleToUpdate.Labels == nil {
			roleToUpdate.Labels = make(map[string]string)
		}
		roleToUpdate.Labels[ownerRefUpdated] = "true"
		_, err := o.roles.Update(roleToUpdate)
		return err
	})

	return nil, retryErr
}

// roleBindingSync updates roles for these resources that have incorrect ownerRefs
func (o *ownerRefCleaner) roleBindingSync(key string, roleBinding *v1.RoleBinding) (runtime.Object, error) {
	if roleBinding == nil || roleBinding.DeletionTimestamp != nil {
		return nil, nil
	}
	if val, ok := roleBinding.Labels[ownerRefUpdated]; ok && val == "true" {
		return nil, nil
	}
	// All these resources are either created in cattle-global-data ns, or in case of node templates in cattle-global-nt ns
	if roleBinding.Namespace != namespaces.GlobalNamespace && roleBinding.Namespace != namespaces.NodeTemplateGlobalNamespace {
		return nil, nil
	}
	if len(roleBinding.OwnerReferences) == 0 {
		return nil, nil
	}

	needsUpdate := correctOwnerRefs(&roleBinding.OwnerReferences)
	if !needsUpdate {
		return nil, nil
	}

	logrus.Infof("Updating ownerReferences of RoleBinding %v/%v", roleBinding.Namespace, roleBinding.Name)

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		rbToUpdate, updateErr := o.roleBindings.GetNamespaced(roleBinding.Namespace, roleBinding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		rbToUpdate.OwnerReferences = roleBinding.OwnerReferences
		if rbToUpdate.Labels == nil {
			rbToUpdate.Labels = make(map[string]string)
		}
		rbToUpdate.Labels[ownerRefUpdated] = "true"
		_, err := o.roleBindings.Update(rbToUpdate)
		return err
	})

	return nil, retryErr
}

func correctOwnerRefs(ownerReferences *[]metav1.OwnerReference) bool {
	var needsUpdate bool
	for ind, ownerRef := range *ownerReferences {
		switch ownerRef.Kind {
		case "secrets":
			/* Cloud credentials (represented by a Secret) had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: v1
			    kind: secrets
			    name: cc-q56qf
			    uid: 709d1cb7-a4a6-422d-8887-d330f977ac49
			Only the kind field is incorrect, need to change it to "Secret"*/
			ownerRef.Kind = typesv1.SecretResource.Kind
			needsUpdate = true
		case "nodetemplates":
			/* Node Templates had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: management.cattle.io
			    kind: nodetemplates
			    name: nt-6wclz
			    uid: fcd6be81-0368-4e0b-a511-705c2bae4d82
			The APIVersion and Kind both fields are incorrect*/
			ownerRef.APIVersion = RancherManagementAPIVersion
			ownerRef.Kind = v3.NodeTemplateGroupVersionKind.Kind
			needsUpdate = true
		case "clustertemplates":
			/* Cluster Templates had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: management.cattle.io
			    kind: clustertemplates
			    name: ct-8thxx
			    uid: a09f047d-f78e-4895-8848-0d12088435d1
			The APIVersion and Kind both fields are incorrect*/
			ownerRef.APIVersion = RancherManagementAPIVersion
			ownerRef.Kind = v3.ClusterTemplateGroupVersionKind.Kind
			needsUpdate = true
		case "clustertemplaterevisions":
			/* Cluster Template Revisions had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: management.cattle.io
			    kind: clustertemplaterevisions
			    name: ctr-98ks7
			    uid: a1f5a67f-44bc-4011-aec6-d40ccfe12442
			The APIVersion and Kind both fields are incorrect*/
			ownerRef.APIVersion = RancherManagementAPIVersion
			ownerRef.Kind = v3.ClusterTemplateRevisionGroupVersionKind.Kind
			needsUpdate = true
		case "globaldnses":
			/* Global DNS entries had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: management.cattle.io
			    kind: globaldnses
			    name: gd-s82dh
			    uid: 3ecf8b69-525b-4a11-85a9-e34cae71f761
			The APIVersion and Kind both fields are incorrect*/
			ownerRef.APIVersion = RancherManagementAPIVersion
			ownerRef.Kind = v3.GlobalDnsGroupVersionKind.Kind
			needsUpdate = true
		case "globaldnsproviders":
			/* Global DNS providers had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: management.cattle.io
			    kind: globaldnsproviders
			    name: rajashree-test
			    uid: 1e1775da-1eb2-4d9b-99d6-9020f7357ee2
			The APIVersion and Kind both fields are incorrect*/
			ownerRef.APIVersion = RancherManagementAPIVersion
			ownerRef.Kind = v3.GlobalDnsProviderGroupVersionKind.Kind
			needsUpdate = true
		case "multiclusterapps":
			/* Multi Cluster Apps had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: management.cattle.io
			    kind: multiclusterapps
			    name: wp
			    uid: fe24c360-d35d-48f6-8295-9a66fdefac79
			The APIVersion and Kind both fields are incorrect*/
			ownerRef.APIVersion = RancherManagementAPIVersion
			ownerRef.Kind = v3.MultiClusterAppGroupVersionKind.Kind
			needsUpdate = true
		case "multiclusterapprevisions":
			/* Multi Cluster Revisions had a Role & RoleBinding created for them with this ownerRef format:
			  - apiVersion: management.cattle.io
			    kind: multiclusterapprevisions
			    name: mcapprevision-mbh68
			    uid: f0c30e41-dbe0-4057-8fac-760b0d8f54e1
			The APIVersion and Kind both fields are incorrect*/
			ownerRef.APIVersion = RancherManagementAPIVersion
			ownerRef.Kind = v3.MultiClusterAppRevisionGroupVersionKind.Kind
			needsUpdate = true
		}
		(*ownerReferences)[ind] = ownerRef
	}
	return needsUpdate
}
