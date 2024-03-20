package dashboard

import (
	"reflect"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	mngtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	rbacv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	adminRole      = "fleetworkspace-admin"
	memberRole     = "fleetworkspace-member"
	readonlyRole   = "fleetworkspace-readonly"
	apiServiceName = "git-webhook"
)

func AddFleetRoles(wrangler *wrangler.Context) error {
	f, err := wrangler.Mgmt.Feature().Get("fleet", metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !features.IsEnabled(f) {
		toDeleteClusterRole := []string{
			adminRole,
			memberRole,
			readonlyRole,
		}
		for _, name := range toDeleteClusterRole {
			if err := wrangler.RBAC.ClusterRole().Delete(name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return err
			}
		}

		if err := wrangler.Mgmt.APIService().Delete(apiServiceName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		}

		return nil
	}

	if err := ensureWebhookAPIService(wrangler.Mgmt.APIService()); err != nil {
		return err
	}

	return ensureFleetRoles(wrangler.RBAC)
}

func ensureWebhookAPIService(apiservices mngtv3.APIServiceClient) error {
	apiService := &v3.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiServiceName,
		},

		Spec: v3.APIServiceSpec{
			SecretName:      "stv-aggregation",
			SecretNamespace: fleetconst.ReleaseNamespace,
			Paths: []string{
				"/fleet/webhook",
			},
		},
	}

	existing, err := apiservices.Get(apiService.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		if _, err := apiservices.Create(apiService); err != nil {
			return err
		}
	} else {
		if !reflect.DeepEqual(existing.Spec, apiService.Spec) {
			existing.Spec = apiService.Spec
			if _, err := apiservices.Update(existing); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureFleetRoles(rbac rbacv1.Interface) error {
	uiLabels := map[string]string{
		"management.cattle.io/ui-product": "fleet",
	}
	fleetWorkspaceAdminRole := v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   adminRole,
			Labels: uiLabels,
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{
					"fleet.cattle.io",
				},
				Resources: []string{
					"clusterregistrationtokens",
					"gitreporestrictions",
					"clusterregistrations",
					"clusters",
					"gitrepos",
					"bundles",
					"clustergroups",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"rbac.authorization.k8s.io",
				},
				Resources: []string{
					"rolebindings",
				},
				Verbs: []string{
					"*",
				},
			},
		},
	}

	fleetWorkspaceMemberRole := v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   memberRole,
			Labels: uiLabels,
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{
					"fleet.cattle.io",
				},
				Resources: []string{
					"gitrepos",
					"bundles",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"fleet.cattle.io",
				},
				Resources: []string{
					"clusterregistrationtokens",
					"gitreporestrictions",
					"clusterregistrations",
					"clusters",
					"clustergroups",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
		},
	}

	fleetWorkspaceReadonlyRole := v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   readonlyRole,
			Labels: uiLabels,
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{
					"fleet.cattle.io",
				},
				Resources: []string{
					"clusterregistrationtokens",
					"gitreporestrictions",
					"clusterregistrations",
					"clusters",
					"gitrepos",
					"bundles",
					"clustergroups",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
		},
	}

	clusterRoles := []v1.ClusterRole{
		fleetWorkspaceAdminRole,
		fleetWorkspaceMemberRole,
		fleetWorkspaceReadonlyRole,
	}

	for _, role := range clusterRoles {
		existing, err := rbac.ClusterRole().Get(role.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		} else if errors.IsNotFound(err) {
			if _, err := rbac.ClusterRole().Create(&role); err != nil {
				return err
			}
		} else {
			if !reflect.DeepEqual(existing.Rules, role.Rules) || !reflect.DeepEqual(existing.Labels, role.Labels) {
				if _, err := rbac.ClusterRole().Update(&role); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
