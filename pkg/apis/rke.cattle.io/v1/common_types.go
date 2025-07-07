package v1

import (
	"encoding/json"

	"github.com/rancher/wrangler/v3/pkg/data/convert"
)

const (
	AuthConfigSecretType = "rke.cattle.io/auth-config"

	UsernameAuthConfigSecretKey      = "username"
	PasswordAuthConfigSecretKey      = "password"
	AuthAuthConfigSecretKey          = "auth"
	IdentityTokenAuthConfigSecretKey = "identityToken"
)

type GenericMap struct {
	Data map[string]any `json:"-"`
}

func (in GenericMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(in.Data)
}

func (in *GenericMap) UnmarshalJSON(data []byte) error {
	in.Data = map[string]any{}
	return json.Unmarshal(data, &in.Data)
}

func (in *GenericMap) DeepCopyInto(out *GenericMap) {
	out.Data = map[string]any{}
	if err := convert.ToObj(in.Data, &out.Data); err != nil {
		panic(err)
	}
}

type LocalClusterAuthEndpoint struct {
	// Enabled indicates whether the local cluster auth endpoint should be
	// enabled.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// FQDN is the fully qualified domain name of the local cluster auth
	// endpoint.
	// +nullable
	// +optional
	FQDN string `json:"fqdn,omitempty"`

	// CACerts is the CA certificate for the local cluster auth endpoint.
	// +nullable
	// +optional
	CACerts string `json:"caCerts,omitempty"`
}

// EnvVar represents a key value pair for an environment variable.
type EnvVar struct {
	// Name is the name of the environment variable.
	Name string `json:"name,omitempty"`

	// Value is the value of the environment variable.
	// +nullable
	// +optional
	Value string `json:"value,omitempty"`
}

type ETCDSnapshotCreate struct {
	// Generation is the current generation for which an etcd snapshot
	// creation operation has been requested.
	// Changing the Generation is the only thing required to create a
	// snapshot.
	// +optional
	Generation int `json:"generation,omitempty"`
}

type ETCDSnapshotRestore struct {
	// Name refers to the name of the associated etcdsnapshot object.
	// +nullable
	// +optional
	Name string `json:"name,omitempty"`

	// Generation is the current generation for which an etcd snapshot
	// restore operation has been requested.
	// Changing the Generation is the only thing required to initiate a
	// snapshot restore.
	// +optional
	Generation int `json:"generation,omitempty"`

	// Set to either none (or empty string), all, or kubernetesVersion
	// +nullable
	// +optional
	RestoreRKEConfig string `json:"restoreRKEConfig,omitempty"`
}

type RotateCertificates struct {
	// Generation is the current generation for which a certificate rotation
	// operation has been requested.
	// Changing the Generation is the only thing required to initiate a
	// certificate rotation.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Services is a list of services to rotate certificates for.
	// If the list is empty, all services will be rotated.
	// +nullable
	// +optional
	Services []string `json:"services,omitempty"`
}

type RotateEncryptionKeys struct {
	// Generation is the current generation for which an encryption key
	// rotation operation has been requested.
	// Changing the Generation is the only thing required to rotate
	// encryption keys.
	// +optional
	Generation int64 `json:"generation,omitempty"`
}
