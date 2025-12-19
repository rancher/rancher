package autoscaler

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	autoscalerTokenTTL = 30 * 24 * time.Hour
)

// ensureUser ensures the user for the given cluster is created or retrieved from cache.
// It first checks if a user with the autoscaler username already exists. If found,
// it returns the existing user. If not found, it creates a new user with the
// appropriate owner references pointing to the cluster.
// Returns the user object or an error if the operation fails.
func (h *autoscalerHandler) ensureUser(cluster *capi.Cluster) (*v3.User, error) {
	u, err := h.userCache.Get(autoscalerUserName(cluster))
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if u != nil {
		return u, err
	}

	user, err := h.userClient.Create(&v3.User{
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

// ensureGlobalRole ensures a GlobalRole is created or updated with appropriate rules for cluster and machine access.
// It gathers all machine deployment and machine resource names, then constructs namespaced rules for write access
// to the cluster namespace and global rules for read access to all CAPI objects. If the role doesn't exist,
// it creates a new one. If it exists but has different rules, it updates the role with the new rules.
// Returns the global role object or an error if the operation fails.
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

	if len(mds) > 0 {
		// grab the machineTemplate info from the first MachineDeployment, the autoscaler needs access to read the
		// machineTemplate object associated with the machineDeployment in order to scale from zero.

		infraType := strings.ToLower(mds[0].Spec.Template.Spec.InfrastructureRef.Kind) + "s"
		infraAPIGroup := strings.Split(mds[0].Spec.Template.Spec.InfrastructureRef.APIVersion, "/")[0]
		namespacedRules[cluster.Namespace] = append(namespacedRules[cluster.Namespace], rbacv1.PolicyRule{
			APIGroups: []string{infraAPIGroup},
			Resources: []string{infraType},
			Verbs:     []string{"get", "list", "watch"},
		})
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

	var (
		globalRole *v3.GlobalRole
		err        error
	)

	// retry on conflict in case the globalrole controller is also modifying this object.
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		globalRole, err = h.globalRoleCache.Get(globalRoleName(cluster))

		// if the role doesn't exist just create it
		if errors.IsNotFound(err) {
			globalRole, err = h.globalRoleClient.Create(
				&v3.GlobalRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:            globalRoleName(cluster),
						OwnerReferences: ownerReference(cluster),
					},
					DisplayName:     fmt.Sprintf("Autoscaler Global Role [%v]", cluster.Name),
					NamespacedRules: namespacedRules,
					Rules:           rules,
				})

			return err
		} else if err == nil {
			// otherwise, check if we need to update the computed rules associated with this cluster
			if !reflect.DeepEqual(globalRole.NamespacedRules, namespacedRules) || !reflect.DeepEqual(globalRole.Rules, rules) {
				globalRole = globalRole.DeepCopy()
				globalRole.NamespacedRules = namespacedRules
				globalRole.Rules = rules
				globalRole, err = h.globalRoleClient.Update(globalRole)

				return err
			}

			return nil
		}

		return err
	})

	return globalRole, err
}

// ensureGlobalRoleBinding ensures a GlobalRoleBinding exists between the specified user and global role.
// It checks if a binding with the same name already exists. If not found, it creates a new binding.
// If found but has different user or role names, it updates the existing binding.
// Returns an error if the create or update operation fails.
func (h *autoscalerHandler) ensureGlobalRoleBinding(cluster *capi.Cluster, username, globalRoleName string) error {
	grb, err := h.globalRoleBindingCache.Get(globalRoleBindingName(cluster))

	if errors.IsNotFound(err) {
		_, err = h.globalRoleBindingClient.Create(&v3.GlobalRoleBinding{
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
			_, err = h.globalRoleBindingClient.Update(grb)
		}
	}

	return err
}

// ensureUserToken ensures a user token is available for the given cluster and username.
// It first checks if a token already exists for the user. If found, it returns the existing token
// in the format "username:token". If not found, it generates a new token with appropriate labels
// and owner references, then creates it in the system.
// Returns the token string in "username:token" format or an error if the operation fails.
func (h *autoscalerHandler) ensureUserToken(cluster *capi.Cluster, username string) (string, error) {
	t, err := h.tokenCache.Get(username)
	if err != nil && !errors.IsNotFound(err) {
		return "", err
	}

	// token already exists - so just return the token string.
	if t != nil {
		return fmt.Sprintf("%s:%s", username, t.Token), nil
	}

	token, err := generateToken(username, cluster.Name, ownerReference(cluster))
	if err != nil {
		return "", err
	}

	_, err = h.tokenClient.Create(token)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", username, token.Token), nil
}

// ensureKubeconfigSecretUsingTemplate ensures a kubeconfig secret exists for the given cluster and token.
// It first checks if a secret with the autoscaler kubeconfig name already exists. If found, it returns
// the existing secret. If not found, it creates a new secret with the generated kubeconfig data,
// appropriate annotations for synchronization, and labels for identification.
// Returns the secret object or an error if the operation fails.
func (h *autoscalerHandler) ensureKubeconfigSecretUsingTemplate(cluster *capi.Cluster, token string) (*corev1.Secret, error) {
	s, err := h.secretCache.Get(cluster.Namespace, kubeconfigSecretName(cluster))
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

	return h.secretClient.Create(secret)
}

// cleanupRBAC removes all autoscaler-related RBAC resources for a given cluster.
// It attempts to delete the user, global role, global role binding, token, and kubeconfig secret
// associated with the cluster. The deletion is performed safely - it only deletes resources
// that exist and collects any errors that occur during the process.
// Returns a combined error if any deletions fail, or nil if all operations succeed.
func (h *autoscalerHandler) cleanupRBAC(cluster *capi.Cluster) error {
	var errs []error

	// Delete the user if it exists
	userName := autoscalerUserName(cluster)
	if _, err := h.userCache.Get(userName); err == nil {
		if err := h.userClient.Delete(userName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete user %s: %w", userName, err))
		}
	} else if !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to check existence of user %s: %w", userName, err))
	}

	// Delete the global role if it exists
	globalRoleName := globalRoleName(cluster)
	if _, err := h.globalRoleCache.Get(globalRoleName); err == nil {
		if err := h.globalRoleClient.Delete(globalRoleName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete global role %s: %w", globalRoleName, err))
		}
	} else if !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to check existence of global role %s: %w", globalRoleName, err))
	}

	// Delete the global role binding if it exists
	globalRoleBindingName := globalRoleBindingName(cluster)
	if _, err := h.globalRoleBindingCache.Get(globalRoleBindingName); err == nil {
		if err := h.globalRoleBindingClient.Delete(globalRoleBindingName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete global role binding %s: %w", globalRoleBindingName, err))
		}
	} else if !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to check existence of global role binding %s: %w", globalRoleBindingName, err))
	}

	// Delete the token if it exists
	if _, err := h.tokenCache.Get(userName); err == nil {
		if err := h.tokenClient.Delete(userName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete token for user %s: %w", userName, err))
		}
	} else if !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to check existence of token for user %s: %w", userName, err))
	}

	// Delete the kubeconfig secret if it exists
	secretName := kubeconfigSecretName(cluster)
	if _, err := h.secretCache.Get(cluster.Namespace, secretName); err == nil {
		if err := h.secretClient.Delete(cluster.Namespace, secretName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete secret %s in namespace %s: %w", secretName, cluster.Namespace, err))
		}
	} else if !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to check existence of secret %s in namespace %s: %w", secretName, cluster.Namespace, err))
	}

	// Return combined errors if any occurred
	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors during cleanup: %v", len(errs), errs)
	}

	return nil
}
