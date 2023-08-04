package kubeconfig

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/moby/locker"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	appcontroller "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/randomtoken"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	userIDLabel     = "authn.management.cattle.io/token-userId"
	tokenKindLabel  = "authn.management.cattle.io/kind"
	tokenHashedAnno = "authn.management.cattle.io/token-hashed"

	hashFormat = "$%d:%s:%s" // $version:salt:hash -> $1:abc:def
	Version    = 2

	ClusterLabelName = "cluster.x-k8s.io/cluster-name"
)

type Manager struct {
	deploymentCache  appcontroller.DeploymentCache
	daemonsetCache   appcontroller.DaemonSetCache
	tokens           mgmtcontrollers.TokenClient
	userCache        mgmtcontrollers.UserCache
	users            mgmtcontrollers.UserClient
	secretCache      corecontrollers.SecretCache
	secrets          corecontrollers.SecretClient
	kubeConfigLocker locker.Locker
}

func New(clients *wrangler.Context) *Manager {
	return &Manager{
		deploymentCache: clients.Apps.Deployment().Cache(),
		daemonsetCache:  clients.Apps.DaemonSet().Cache(),
		tokens:          clients.Mgmt.Token(),
		userCache:       clients.Mgmt.User().Cache(),
		users:           clients.Mgmt.User(),
		secretCache:     clients.Core.Secret().Cache(),
		secrets:         clients.Core.Secret(),
	}
}

func getKubeConfigSecretName(clusterName string) string {
	return clusterName + "-kubeconfig"
}

func (m *Manager) getToken(clusterNamespace, clusterName string) (string, error) {
	kubeConfigSecretName := getKubeConfigSecretName(clusterName)
	if token, err := m.getSavedToken(clusterNamespace, kubeConfigSecretName); err != nil || token != "" {
		return token, err
	}

	// Need to be careful about caches being out of sync since we are dealing with multiple objects that
	// arent eventually consistent (because we delete and create the token for the user)
	if token, err := m.getSavedTokenNoCache(clusterNamespace, kubeConfigSecretName); err != nil || token != "" {
		return token, err
	}

	userName, err := m.EnsureUser(clusterNamespace, clusterName)
	if err != nil {
		return "", err
	}

	return m.createUserToken(userName)
}

func (m *Manager) EnsureUser(clusterNamespace, clusterName string) (string, error) {
	principalID := getPrincipalID(clusterNamespace, clusterName)
	userName := getUserNameForPrincipal(principalID)
	return userName, m.createUser(principalID, userName)
}

func getUserNameForPrincipal(principal string) string {
	hasher := sha256.New()
	hasher.Write([]byte(principal))
	sha := base32.StdEncoding.WithPadding(-1).EncodeToString(hasher.Sum(nil))[:10]
	return "u-" + strings.ToLower(sha)
}

func labelsForUser(principalID string) map[string]string {
	encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(principalID))
	if len(encodedPrincipalID) > 63 {
		encodedPrincipalID = encodedPrincipalID[:63]
	}
	return map[string]string{
		encodedPrincipalID: "hashed-principal-name",
	}
}

func (m *Manager) getSavedToken(kubeConfigNamespace, kubeConfigName string) (string, error) {
	secret, err := m.secretCache.Get(kubeConfigNamespace, kubeConfigName)
	if apierror.IsNotFound(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return string(secret.Data["token"]), nil
}

func (m *Manager) getSavedTokenNoCache(kubeConfigNamespace, kubeConfigName string) (string, error) {
	secret, err := m.secrets.Get(kubeConfigNamespace, kubeConfigName, metav1.GetOptions{})
	if apierror.IsNotFound(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return string(secret.Data["token"]), nil
}

func getPrincipalID(clusterNamespace, clusterName string) string {
	return fmt.Sprintf("system://provisioning/%s/%s", clusterNamespace, clusterName)
}

func (m *Manager) createUser(principalID, userName string) error {
	_, err := m.userCache.Get(userName)
	if apierror.IsNotFound(err) {
		_, err = m.users.Create(&v3.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:   userName,
				Labels: labelsForUser(principalID),
			},
			PrincipalIDs: []string{
				principalID,
			},
		})
	}
	return err
}

func (m *Manager) createUserToken(userName string) (string, error) {
	_, err := m.tokens.Get(userName, metav1.GetOptions{})
	if err == nil {
		err = m.tokens.Delete(userName, nil)
	}
	if err != nil && !apierror.IsNotFound(err) {
		return "", err
	}

	tokenValue, err := randomtoken.Generate()
	if err != nil {
		return "", fmt.Errorf("failed to generate token key: %w", err)
	}

	token := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: userName,
			Labels: map[string]string{
				userIDLabel:    userName,
				tokenKindLabel: "provisioning",
			},
			Annotations: map[string]string{},
		},
		UserID:       userName,
		AuthProvider: "local",
		IsDerived:    true,
		Token:        tokenValue,
	}

	if features.TokenHashing.Enabled() {
		tokenHash, err := createSHA256Hash(tokenValue)
		if err != nil {
			return "", err
		}
		token.Token = tokenHash
		token.Annotations[tokenHashedAnno] = "true"
	}

	_, err = m.tokens.Create(token)
	return fmt.Sprintf("%s:%s", userName, tokenValue), err
}

func createSHA256Hash(secretKey string) (string, error) {
	salt := make([]byte, 8)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", salt, secretKey)))
	encSalt := base64.RawStdEncoding.EncodeToString(salt)
	encKey := base64.RawStdEncoding.EncodeToString(hash[:])
	return fmt.Sprintf(hashFormat, Version, encSalt, encKey), nil
}

func (m *Manager) GetCRTBForAdmin(cluster *v1.Cluster, status v1.ClusterStatus) (*v3.ClusterRoleTemplateBinding, error) {
	if status.ClusterName == "" {
		return nil, fmt.Errorf("management cluster is not assigned to v1.Cluster")
	}
	principalID := getPrincipalID(cluster.Namespace, cluster.Name)
	userName := getUserNameForPrincipal(principalID)
	return &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeConcatName(status.ClusterName, "owner"),
			Namespace: status.ClusterName,
		},
		ClusterName:       status.ClusterName,
		UserName:          userName,
		UserPrincipalName: principalID,
		RoleTemplateName:  "cluster-owner",
	}, nil
}

func (m *Manager) getKubeConfigData(cluster *v1.Cluster, secretName, managementClusterName string) (map[string][]byte, error) {
	secret, err := m.secretCache.Get(cluster.Namespace, secretName)
	if err == nil {
		if secret.Data == nil || secret.Data["token"] == nil || len(secret.OwnerReferences) == 0 {
			// Check if we require a new secret based on the token value and annotation(s). We delete the old secret since it may contain
			// annotations, owner references, etc. that are out of date. We will then continue to create the new secret.
			if err := m.secrets.Delete(cluster.Namespace, secretName, &metav1.DeleteOptions{}); err != nil && !apierror.IsNotFound(err) {
				return nil, err
			}
		} else {
			return secret.Data, nil
		}
	} else if !apierror.IsNotFound(err) {
		return nil, err
	}

	lockID := cluster.Namespace + "/" + cluster.Name
	m.kubeConfigLocker.Lock(lockID)
	defer m.kubeConfigLocker.Unlock(lockID)

	secret, err = m.secrets.Get(cluster.Namespace, secretName, metav1.GetOptions{})
	if err == nil {
		return secret.Data, nil
	}

	tokenValue, err := m.getToken(cluster.Namespace, cluster.Name)
	if err != nil {
		return nil, err
	}

	serverURL, cacert := settings.InternalServerURL.Get(), settings.InternalCACerts.Get()
	if serverURL == "" {
		return nil, errors.New("server url is missing, can't generate kubeconfig for fleet import cluster")
	}

	data, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				Server:                   fmt.Sprintf("%s/k8s/clusters/%s", serverURL, managementClusterName),
				CertificateAuthorityData: []byte(strings.TrimSpace(cacert)),
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				Token: tokenValue,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  "cluster",
				AuthInfo: "user",
			},
		},
		CurrentContext: "default",
	})
	if err != nil {
		return nil, err
	}

	secret, err = m.secrets.Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      secretName,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "provisioning.cattle.io/v1",
				Kind:       "Cluster",
				Name:       cluster.Name,
				UID:        cluster.UID,
			}},
			Labels: map[string]string{
				ClusterLabelName: cluster.Name,
			},
		},
		Data: map[string][]byte{
			"value": data,
			"token": []byte(tokenValue),
		},
	})
	if err != nil {
		return nil, err
	}

	return secret.Data, nil
}

func (m *Manager) GetRESTConfig(cluster *v1.Cluster, status v1.ClusterStatus) (*rest.Config, error) {
	secret, err := m.GetKubeConfig(cluster, status)
	if err != nil {
		return nil, err
	}

	return clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
}

func (m *Manager) GetKubeConfig(cluster *v1.Cluster, status v1.ClusterStatus) (*corev1.Secret, error) {
	if cluster.Spec.ClusterAPIConfig != nil {
		return m.secretCache.Get(cluster.Namespace, getKubeConfigSecretName(cluster.Spec.ClusterAPIConfig.ClusterName))
	}

	var (
		secretName = getKubeConfigSecretName(cluster.Name)
	)

	data, err := m.getKubeConfigData(cluster, secretName, status.ClusterName)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      secretName,
		},
		Data: data,
	}, nil
}
