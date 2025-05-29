package util

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/version"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	RegCodeInitializerNameKey             = "regCodeSecretName"
	RegCodeInitializerNamespaceKey        = "regCodeSecretNamespace"
	RegCodeSecretName                     = "rancher-scc-registration-code"
	RegCodeSecretKey                      = "regCode"
	RegCertInitializerNameKey             = "certificateSecretName"
	RegCertInitializerNamespaceKey        = "certificateSecretNamespace"
	RegCertSecretName                     = "rancher-scc-registration-certificate"
	RegCertSecretKey                      = "certificate"
	RancherSCCSystemCredentialsSecretName = "rancher-scc-system-credentials"
	RancherSCCOfflineRequestSecretName    = "rancher-scc-offline-registration-request"
)

func ValidateInitializingConfigMap(sccInitializerConfig *corev1.ConfigMap) (*corev1.SecretReference, *v1.RegistrationMode, error) {
	secretReference := &corev1.SecretReference{}
	// Verify the expected fields are on the config map
	modeString, _ := sccInitializerConfig.Data["mode"]
	mode := v1.RegistrationMode(modeString)
	if !mode.Valid() {
		errorMsg := fmt.Sprintf("the configmap does not have a valid mode set")
		logrus.Error(errorMsg)
		return secretReference, nil, errors.New(errorMsg)
	}

	credentialNameKey := ""
	credentialNamespaceKey := ""
	if mode == v1.RegistrationModeOnline {
		credentialNameKey = RegCodeInitializerNameKey
		credentialNamespaceKey = RegCodeInitializerNamespaceKey
	} else {
		credentialNameKey = RegCertInitializerNameKey
		credentialNamespaceKey = RegCertInitializerNamespaceKey
	}

	secretName, credOk := sccInitializerConfig.Data[credentialNameKey]
	if !credOk {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		errorMsg := fmt.Sprintf("cannot find the credential value key %s", credentialNameKey)
		logrus.Error(errorMsg)
		return secretReference, nil, errors.New(errorMsg)
	}

	secretNamespace, credOk := sccInitializerConfig.Data[credentialNamespaceKey]
	if !credOk {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		errorMsg := fmt.Sprintf("cannot find the credential value key %s", credentialNamespaceKey)
		logrus.Error(errorMsg)
		secretNamespace = "cattle-system"
	}

	secretReference.Name = secretName
	secretReference.Namespace = secretNamespace

	return secretReference, &mode, nil
}

func VersionIsDevBuild() bool {
	if version.Version == "dev" {
		return true
	}

	// TODO replace with regex
	parsedVer, err := semver.NewVersion(version.Version)
	if err != nil {
		logrus.Errorf("Error parsing version %s: %s", version.Version, err)
		logrus.Warnf("The version will be interpreted as a development build")
		return true
	}
	return parsedVer.Prerelease() != ""
}
