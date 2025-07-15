package autoscaler

import (
	"fmt"
	"reflect"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/pkg/randomtoken"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	userIDLabel          = "authn.management.cattle.io/token-userId"
	tokenKindLabel       = "authn.management.cattle.io/kind"
	autoscalerTokenTTL   = 30 * 24 * time.Hour
	renewalCheckInterval = 24 * time.Hour
	renewalThreshold     = 7 * 24 * time.Hour
)

func (h *autoscalerHandler) ensureUser(cluster *capi.Cluster) (*v3.User, error) {
	u, err := h.userCache.Get(autoscalerUserName(cluster))
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if u != nil {
		return u, err
	}

	user, err := h.user.Create(&v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:            autoscalerUserName(cluster),
			OwnerReferences: ownerReference(cluster),
		},
		Username: autoscalerUserName(cluster),
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (h *autoscalerHandler) ensureGlobalRole(cluster *capi.Cluster, mds []*capi.MachineDeployment, machines []*capi.Machine) (*v3.GlobalRole, error) {
	mdResourceNames := make([]string, len(mds))
	machineResourceNames := make([]string, len(machines))

	// gather up all the pools that this globalRole needs to be attached to
	for i, md := range mds {
		mdResourceNames[i] = md.Name
	}

	// also gather up all the machines that this globalRole needs access to
	for i, machine := range machines {
		machineResourceNames[i] = machine.Name
	}

	role, err := h.globalRoleCache.Get(globalRoleName(cluster))
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	wanted := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            globalRoleName(cluster),
			OwnerReferences: ownerReference(cluster),
		},
		DisplayName: fmt.Sprintf("Autoscaler Global Role [%v]", cluster.Name),
		// scope write-related rules to the namespace the capi resources are in
		NamespacedRules: map[string][]rbacv1.PolicyRule{
			cluster.Namespace: {
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					Resources:     []string{"machinedeployments"},
					Verbs:         []string{"get", "update", "patch"},
					ResourceNames: mdResourceNames,
				},
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					Resources:     []string{"machinedeployments/scale"},
					Verbs:         []string{"get", "update", "patch"},
					ResourceNames: mdResourceNames,
				},
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					Resources:     []string{"machines"},
					Verbs:         []string{"get", "update", "patch"},
					ResourceNames: machineResourceNames,
				},
			},
		},
		// clusterrole for read-access to all capi objects is required
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"cluster.x-k8s.io"},
				Resources: []string{
					"machinedeployments",
					"machinepools",
					"machines",
					"machinesets",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}

	// if the role doesn't exist just create it and return that error
	if errors.IsNotFound(err) {
		return h.globalRole.Create(wanted)
	}

	// Role exists, check if mdResourceNames need to be updated
	// TODO: update this to be a bit cleaner. feels yuck.
	if !reflect.DeepEqual(role.NamespacedRules, wanted.NamespacedRules) ||
		!reflect.DeepEqual(role.Rules, wanted.Rules) {
		// Update the existing role with new mdResourceNames
		updatedRole := role.DeepCopy()
		updatedRole.Rules = wanted.Rules
		updatedRole.NamespacedRules = wanted.NamespacedRules

		return h.globalRole.Update(updatedRole)
	}

	// Resource names match, no update needed
	return role, nil
}

func (h *autoscalerHandler) ensureGlobalRoleBinding(cluster *capi.Cluster, username, globalRoleName string) error {
	rb, err := h.globalRoleBindingCache.Get(globalRoleBindingName(cluster))
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// binding already exists - don't try to recreate it.
	if rb != nil {
		return nil
	}

	_, err = h.globalRoleBinding.Create(&v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            globalRoleBindingName(cluster),
			OwnerReferences: ownerReference(cluster),
		},
		GlobalRoleName: globalRoleName,
		UserName:       username,
	})

	return err
}

func (h *autoscalerHandler) ensureUserToken(cluster *capi.Cluster, username string) (string, error) {
	t, err := h.tokenCache.Get(username)
	if err != nil && !errors.IsNotFound(err) {
		return "", err
	}

	// token already exists - so just return the token string.
	if t != nil {
		return fmt.Sprintf("%s:%s", username, t.Token), err
	}

	tokenValue, err := randomtoken.Generate()
	if err != nil {
		return "", fmt.Errorf("failed to generate token key: %w", err)
	}

	token := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: username,
			Labels: map[string]string{
				userIDLabel:           username,
				tokenKindLabel:        "autoscaler",
				capi.ClusterNameLabel: cluster.Name,
			},
			Annotations:     map[string]string{},
			OwnerReferences: ownerReference(cluster),
		},
		UserID:       username,
		AuthProvider: "local",
		IsDerived:    true,
		Token:        tokenValue,
		TTLMillis:    autoscalerTokenTTL.Milliseconds(),
	}

	if features.TokenHashing.Enabled() {
		err := tokens.ConvertTokenKeyToHash(token)
		if err != nil {
			return "", fmt.Errorf("unable to hash token: %w", err)
		}
	}

	_, err = h.token.Create(token)
	return fmt.Sprintf("%s:%s", username, tokenValue), err
}

// createKubeConfigSecretUsingTemplate creates a kubeconfig secret string given a cluster and token
func (h *autoscalerHandler) createKubeConfigSecretUsingTemplate(cluster *capi.Cluster, token string) error {
	s, err := h.secretsCache.Get(cluster.Namespace, kubeconfigSecretName(cluster))
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if s != nil {
		return nil
	}

	data, err := generateKubeconfig(token)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       cluster.Namespace,
			Name:            kubeconfigSecretName(cluster),
			OwnerReferences: ownerReference(cluster),
			Annotations: map[string]string{
				"provisioning.cattle.io/sync":                  "true",
				"provisioning.cattle.io/sync-target-namespace": "kube-system",
				"provisioning.cattle.io/sync-target-name":      "mgmt-kubeconfig",
				"rke.cattle.io/object-authorized-for-clusters": cluster.Name,
			},
			Labels: map[string]string{
				capi.ClusterNameLabel:                    cluster.Name,
				"provisioning.cattle.io/kubeconfig-type": "autoscaler",
			},
		},
		Data: map[string][]byte{
			"value": data,
			"token": []byte(token),
		},
	}

	_, err = h.secrets.Create(secret)
	return err
}
