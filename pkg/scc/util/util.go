package util

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/rancher/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"math/rand"
	"regexp"
	"time"
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

func utilContextLogger() log.StructuredLogger {
	return log.NewLog().WithField("subcomponent", "util")
}

// ValidateInitializingConfigMap TODO: repurpose for validate secret?
func ValidateInitializingConfigMap(sccInitializerConfig *corev1.ConfigMap) (*corev1.SecretReference, *v1.RegistrationMode, error) {
	secretReference := &corev1.SecretReference{}
	// Verify the expected fields are on the config map
	modeString, _ := sccInitializerConfig.Data["mode"]
	mode := v1.RegistrationMode(modeString)
	if !mode.Valid() {
		errorMsg := fmt.Sprintf("the configmap does not have a valid mode set")
		utilContextLogger().Error(errorMsg)
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
		utilContextLogger().Error(errorMsg)
		return secretReference, nil, errors.New(errorMsg)
	}

	secretNamespace, credOk := sccInitializerConfig.Data[credentialNamespaceKey]
	if !credOk {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		errorMsg := fmt.Sprintf("cannot find the credential value key %s", credentialNamespaceKey)
		utilContextLogger().Error(errorMsg)
		secretNamespace = "cattle-system"
	}

	secretReference.Name = secretName
	secretReference.Namespace = secretNamespace

	return secretReference, &mode, nil
}

// semverRegex matches on regular SemVer as well as Rancher's dev versions
var semverRegex = regexp.MustCompile(`(?m)^v?(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)(?:\.(?P<patch>0|[1-9]\d*))?(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

func VersionIsDevBuild() bool {
	rancherVersion := version.Version
	if rancherVersion == "dev" {
		return true
	}

	matches := semverRegex.FindStringSubmatch(rancherVersion)
	return matches == nil || // When version is not SemVer it is dev
		matches[3] == "" || // When the version is just Major.Minor assume dev
		matches[4] != "" // When the version includes pre-release assume dev
}

func JSONToBase64(data interface{}) ([]byte, error) {
	var jsonData []byte
	var err error

	if b, ok := data.([]byte); ok {
		jsonData = b
	} else {
		jsonData, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON data: %w", err)
		}
	}

	encodedLen := base64.StdEncoding.EncodedLen(len(jsonData))
	output := make([]byte, encodedLen)

	// Base64 encode the JSON byte slice
	base64.StdEncoding.Encode(output, jsonData)

	return output, nil
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type SaltGen struct {
	randSrc     *rand.Rand
	saltCharset string
	charsetLen  int
}

func NewSaltGen(timeIn *time.Time, charsetIn *string) *SaltGen {
	if timeIn == nil {
		now := time.Now()
		timeIn = &now
	}
	randSrc := rand.New(rand.NewSource(timeIn.UnixNano()))

	setCharset := charset
	if charsetIn != nil {
		setCharset = *charsetIn
	}

	return &SaltGen{randSrc: randSrc, saltCharset: setCharset, charsetLen: len(setCharset)}
}

func (s *SaltGen) GenerateCharacter() uint8 {
	randIndex := s.randSrc.Intn(s.charsetLen)
	return s.saltCharset[randIndex]
}

func (s *SaltGen) GenerateSalt() string {
	salt := make([]byte, 8)
	for i := range salt {
		salt[i] = s.GenerateCharacter()
	}

	return string(salt)
}
