package globalrolesv2

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	roleOwner      = "cluster-owner"
	standardUser   = "user"
	localcluster   = "local"
	crtbOwnerLabel = "authz.management.cattle.io/grb-owner"
)

var globalRole = v3.GlobalRole{
	ObjectMeta: metav1.ObjectMeta{
		Name: "",
	},
	InheritedClusterRoles: []string{},
}
