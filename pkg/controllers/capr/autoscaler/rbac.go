package autoscaler

import (
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	autoscalerTokenTTL = 30 * 24 * time.Hour
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

	// scope write-related rules to the namespace the capi resources are in
	namespacedRules := map[string][]rbacv1.PolicyRule{
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
	}

	// clusterrole for read-access to all capi objects is required
	rules := []rbacv1.PolicyRule{
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
	}

	globalRole, err := h.globalRoleCache.Get(globalRoleName(cluster))

	// if the role doesn't exist just create it
	if errors.IsNotFound(err) {
		return h.globalRole.Create(
			&v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:            globalRoleName(cluster),
					OwnerReferences: ownerReference(cluster),
				},
				DisplayName:     fmt.Sprintf("Autoscaler Global Role [%v]", cluster.Name),
				NamespacedRules: namespacedRules,
				Rules:           rules,
			})
	} else if err == nil {
		// otherwise, update the computed rules associated with this cluster
		globalRole = globalRole.DeepCopy()
		globalRole.NamespacedRules = namespacedRules
		globalRole.Rules = rules
		return h.globalRole.Update(globalRole)
	} else {
		return nil, err
	}
}

func (h *autoscalerHandler) ensureGlobalRoleBinding(cluster *capi.Cluster, username, globalRoleName string) error {
	grb, err := h.globalRoleBindingCache.Get(globalRoleBindingName(cluster))

	if errors.IsNotFound(err) {
		_, err = h.globalRoleBinding.Create(&v3.GlobalRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            globalRoleBindingName(cluster),
				OwnerReferences: ownerReference(cluster),
			},
			GlobalRoleName: globalRoleName,
			UserName:       username,
		})
	} else if err == nil {
		if grb.UserName != username || grb.GlobalRoleName != globalRoleName {
			grb = grb.DeepCopy()
			grb.UserName = username
			grb.GlobalRoleName = globalRoleName
			_, err = h.globalRoleBinding.Update(grb)
			return err
		}
	}

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

	token, err := generateToken(username, cluster.Name, ownerReference(cluster))
	if err != nil {
		return "", err
	}

	_, err = h.token.Create(token)
	return fmt.Sprintf("%s:%s", username, token.Token), err
}

// createKubeConfigSecretUsingTemplate creates a kubeconfig secret string given a cluster and token
func (h *autoscalerHandler) createKubeConfigSecretUsingTemplate(cluster *capi.Cluster, token string) (*corev1.Secret, error) {
	s, err := h.secretsCache.Get(cluster.Namespace, kubeconfigSecretName(cluster))
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if s != nil {
		return s, nil
	}

	data, err := generateKubeconfig(token)
	if err != nil {
		return nil, err
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

	return h.secrets.Create(secret)
}
