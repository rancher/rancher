package kubeconfig

import (
	"bytes"
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
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	appcontroller "github.com/rancher/wrangler/v2/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v2/pkg/name"
	"github.com/rancher/wrangler/v2/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	userIDLabel    = "authn.management.cattle.io/token-userId"
	tokenKindLabel = "authn.management.cattle.io/kind"

	hashFormat = "$%d:%s:%s" // $version:salt:hash -> $1:abc:def
	Version    = 2
)

type Manager struct {
	deploymentCache  appcontroller.DeploymentCache
	daemonsetCache   appcontroller.DaemonSetCache
	tokens           mgmtcontrollers.TokenClient
	tokensCache      mgmtcontrollers.TokenCache
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
		tokensCache:     clients.Mgmt.Token().Cache(),
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

// getCachedToken retrieves the token for a given cluster without manipulating the token itself. This function is
// primary to ensure the kubeconfig for a cluster remains valid.
func (m *Manager) getCachedToken(clusterNamespace, clusterName string) (string, *v3.Token, error) {
	_, userName := getPrincipalAndUserName(clusterNamespace, clusterName)
	token, err := m.tokensCache.Get(userName)
	if err != nil {
		return "", nil, err
	}
	return userName, token, nil
}

func getPrincipalAndUserName(clusterNamespace, clusterName string) (string, string) {
	principalID := getPrincipalID(clusterNamespace, clusterName)
	return principalID, getUserNameForPrincipal(principalID)
}

func (m *Manager) EnsureUser(clusterNamespace, clusterName string) (string, error) {
	principalID, userName := getPrincipalAndUserName(clusterNamespace, clusterName)
	return userName, m.createUser(principalID, userName)
}

func (m *Manager) DeleteUser(clusterNamespace, clusterName string) error {
	return m.deleteUser(getUserNameForPrincipal(getPrincipalID(clusterNamespace, clusterName)))
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

func (m *Manager) deleteUser(userName string) error {
	if err := m.users.Delete(userName, nil); err != nil && !apierror.IsNotFound(err) {
		return err
	}
	return nil
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
		err := tokens.ConvertTokenKeyToHash(token)
		if err != nil {
			return "", fmt.Errorf("unable to hash token: %w", err)
		}
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

func (m *Manager) GetCRTBForClusterOwner(cluster *v1.Cluster, status v1.ClusterStatus) (*v3.ClusterRoleTemplateBinding, error) {
	if status.ClusterName == "" {
		return nil, fmt.Errorf("management cluster is not assigned to v1.Cluster")
	}
	principalID := getPrincipalID(cluster.Namespace, cluster.Name)
	userName := getUserNameForPrincipal(principalID)
	return &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeConcatName(status.ClusterName, cluster.Namespace, "owner"),
			Namespace: status.ClusterName,
		},
		ClusterName:       status.ClusterName,
		UserName:          userName,
		UserPrincipalName: principalID,
		RoleTemplateName:  "cluster-owner",
	}, nil
}

// kubeConfigValid accepts a kubeconfig and corresponding data, and validates that the kubeconfig is valid for the
// cluster in question. It returns two booleans, the first of which is whether an error occurred parsing the kubeconfig
// or retrieving information related to the kubeconfig, and the second which indicates whether the kubeconfig is valid.
func (m *Manager) kubeConfigValid(kcData []byte, cluster *v1.Cluster, currentServerURL, currentServerCA, currentManagementClusterName string, secretLabels map[string]string) (bool, bool) {
	if len(kcData) == 0 {
		return true, false
	}
	kc, err := clientcmd.Load(kcData)
	if err != nil {
		logrus.Errorf("error while loading kubeconfig in kubeconfigmanager for validation: %v", err)
		return true, false
	}
	var serverURL, managementCluster string
	splitServer := strings.Split(kc.Clusters["cluster"].Server, "/k8s/clusters/")
	if len(splitServer) != 2 {
		return true, false
	}

	serverURL = splitServer[0]
	managementCluster = splitServer[1]
	logrus.Tracef("[kubeconfigmanager] cluster %s/%s: parsed serverURL: %s and managementServer: %s from existing kubeconfig", cluster.Namespace, cluster.Name, serverURL, managementCluster)

	userName, token, err := m.getCachedToken(cluster.Namespace, cluster.Name)
	if err != nil {
		logrus.Errorf("error while retrieving cached token in kubeconfigmanager for validation: %v", err)
		return true, false
	}

	tokenMatches := fmt.Sprintf("%s:%s", userName, token.Token) == kc.AuthInfos["user"].Token
	if token.Annotations[tokens.TokenHashed] == "true" {
		// if tokenHashing is enabled, the stored token will be hashed. So we instead make sure it's up-to-date by checking if the token is valid for the hash
		hasher, err := hashers.GetHasherForHash(token.Token)
		if err != nil {
			logrus.Errorf("[kubeconfigmanager] error when retrieving hasher for token hash, %s", err.Error())
			return true, false
		}
		_, tokenKey := tokens.SplitTokenParts(kc.AuthInfos["user"].Token)
		err = hasher.VerifyHash(token.Token, tokenKey)
		tokenMatches = err == nil
	}

	// Check if the required CAPI cluster label is present in the secretLabels map
	capiClusterLabelValue, labelPresent := secretLabels[capi.ClusterNameLabel]
	if !labelPresent || capiClusterLabelValue != cluster.Name {
		logrus.Tracef("[kubeconfigmanager] cluster %s/%s: kubeconfig secret failed validation due to missing or incorrect label", cluster.Namespace, cluster.Name)
		return false, false
	}

	if serverURL != currentServerURL || !bytes.Equal([]byte(strings.TrimSpace(currentServerCA)), kc.Clusters["cluster"].CertificateAuthorityData) || managementCluster != currentManagementClusterName || !tokenMatches {
		logrus.Tracef("[kubeconfigmanager] cluster %s/%s: kubeconfig secret failed validation, did not match provided data", cluster.Namespace, cluster.Name)
		return false, false
	}
	logrus.Tracef("[kubeconfigmanager] cluster %s/%s: kubeconfig secret passed validation", cluster.Namespace, cluster.Name)
	return false, true
}

func (m *Manager) getKubeConfigData(cluster *v1.Cluster, secretName, managementClusterName string) (map[string][]byte, error) {
	serverURL, cacert := settings.InternalServerURL.Get(), settings.InternalCACerts.Get()
	if serverURL == "" {
		return nil, errors.New("server url is missing, can't generate kubeconfig for fleet import cluster")
	}

	secret, err := m.secretCache.Get(cluster.Namespace, secretName)
	if err == nil {
		retrievalError, isValid := m.kubeConfigValid(secret.Data["value"], cluster, serverURL, cacert, managementClusterName, secret.Labels)
		if (!retrievalError && !isValid) || secret.Data == nil || secret.Data["token"] == nil || len(secret.OwnerReferences) == 0 {
			logrus.Infof("[kubeconfigmanager] deleting kubeconfig secret for cluster %s/%s", cluster.Namespace, cluster.Name)
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
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "Cluster",
				Name:       cluster.Name,
				UID:        cluster.UID,
			}},
			Labels: map[string]string{
				capi.ClusterNameLabel: cluster.Name,
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
