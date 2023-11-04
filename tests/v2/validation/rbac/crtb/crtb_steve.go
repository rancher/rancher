package crtb

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	crtbAPIEndPoint                 = "management.cattle.io.clusterroletemplatebinding"
	apiVersion                      = "management.cattle.io/v3"
	kind                            = "ClusterRoleTemplateBinding"
	localCluster                    = "local"
	clusterOwner                    = "cluster-owner"
	clusterMember                   = "cluster-member"
	crtbManage                      = "clusterroletemplatebindings-manage"
	crtbNameWithTenChar             = 10
	crtbConflictError               = "409 Conflict"
	crtbAlreadyExisted              = "AlreadyExists"
	badRequest                      = "400 Bad Request"
	notFound                        = "404 Not Found"
	cluserNameAndNamespaceSameValue = "Forbidden: clusterName and namespace must be the same value"
)

func clusterRoleTemplateBindingTemplate(clusterID string, crtbName string, newUser string, roleTemplateName string) v3.ClusterRoleTemplateBinding {
	crtbTemplate := v3.ClusterRoleTemplateBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterID,
			Name:      crtbName,
		},
		ClusterName:      clusterID,
		RoleTemplateName: roleTemplateName,
		UserName:         newUser,
	}

	return crtbTemplate
}
