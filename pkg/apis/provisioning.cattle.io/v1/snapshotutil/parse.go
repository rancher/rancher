package snapshotutil

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
)

const (
	metaPrefix    = "reading snapshot metadata"
	metaMapPrefix = "reading snapshot metadata map"
)

// CompressInterface is a function that will marshal, gzip, then base64 encode the provided interface.
func CompressInterface(v interface{}) (string, error) {
	marshalledCluster, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(marshalledCluster); err != nil {
		return "", err
	}
	if err := gz.Flush(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

// DecompressInterface is a function that will base64 decode, ungzip, and unmarshal a string into the provided interface.
func DecompressInterface(inputb64 string, v any) error {
	if inputb64 == "" {
		return fmt.Errorf("base64 input is empty")
	}

	decodedGzip, err := base64.StdEncoding.DecodeString(inputb64)
	if err != nil {
		return fmt.Errorf("base64 decode failed: %w", err)
	}

	gzr, err := gzip.NewReader(bytes.NewBuffer(decodedGzip))
	if err != nil {
		return fmt.Errorf("gzip decompress failed: %w", err)
	}
	defer gzr.Close()

	csBytes, err := io.ReadAll(gzr)
	if err != nil {
		return fmt.Errorf("gzip read failed: %w", err)
	}

	if err := json.Unmarshal(csBytes, v); err != nil {
		return fmt.Errorf("JSON unmarshal failed: %w", err)
	}
	return nil
}

// DecompressClusterSpec is a function that will base64 decode, ungzip, and unmarshal a string into a cluster spec.
func DecompressClusterSpec(inputb64 string) (*provv1.ClusterSpec, error) {
	c := provv1.ClusterSpec{}
	if err := DecompressInterface(inputb64, &c); err != nil {
		return nil, fmt.Errorf("reading snapshot metadata into ClusterSpec: %w", err)
	}
	return &c, nil
}

// ParseSnapshotClusterSpecOrError returns a provv1 ClusterSpec from the etcd snapshot
// if it can be found in the CR. If it cannot be found, it returns an error.
func ParseSnapshotClusterSpecOrError(snapshot *rkev1.ETCDSnapshot) (*provv1.ClusterSpec, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("%s: snapshot was nil", metaPrefix)
	}

	if snapshot.SnapshotFile.Metadata == "" {
		return nil, fmt.Errorf("%s: metadata map is empty; %q missing", metaPrefix, rkev1.SnapshotMetadataClusterSpecKey)
	}

	b, err := base64.StdEncoding.DecodeString(snapshot.SnapshotFile.Metadata)
	if err != nil {
		return nil, fmt.Errorf("%s: base64 decode failed: %w", metaMapPrefix, err)
	}

	var md map[string]string
	if err := json.Unmarshal(b, &md); err != nil {
		return nil, fmt.Errorf("%s: JSON unmarshal failed: %w", metaMapPrefix, err)
	}

	raw, ok := md[rkev1.SnapshotMetadataClusterSpecKey]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s: %q key not found or empty in snapshot metadata", metaPrefix, rkev1.SnapshotMetadataClusterSpecKey)
	}

	spec, err := DecompressClusterSpec(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to decode %q payload: %w", metaPrefix, rkev1.SnapshotMetadataClusterSpecKey, err)
	}

	return spec, nil
}

// RestoreModeRequiresClusterSpec reports whether the given restore mode requires a
// valid provisioning ClusterSpec to be present and decodable from the snapshot.
func RestoreModeRequiresClusterSpec(r *rkev1.ETCDSnapshotRestore) bool {
	if r == nil {
		return false
	}
	// If no snapshot is named yet, we treat it as not requiring spec (no-op for planner gating).
	if r.Name == "" {
		return false
	}
	mode := r.RestoreRKEConfig
	if mode == "" || mode == rkev1.RestoreRKEConfigNone {
		return false
	}
	return mode == rkev1.RestoreRKEConfigKubernetesVersion || mode == rkev1.RestoreRKEConfigAll
}
