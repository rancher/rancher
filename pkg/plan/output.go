package plan

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
)

// ReadAppliedOutput decodes the gzip-compressed one-time instruction output from a plan secret.
// Returns nil if no output has been written yet. Keys in the returned map correspond to
// instruction names that were assigned with SaveOutput: true.
func ReadAppliedOutput(secret *corev1.Secret) (map[string][]byte, error) {
	raw := secret.Data["applied-output"]
	if len(raw) == 0 {
		return nil, nil
	}
	decoded, err := decompressGzip(raw)
	if err != nil {
		return nil, fmt.Errorf("reading applied-output from %s: %w", secret.Name, err)
	}
	var out map[string][]byte
	if err := json.Unmarshal(decoded, &out); err != nil {
		return nil, fmt.Errorf("parsing applied-output from %s: %w", secret.Name, err)
	}
	return out, nil
}

// ReadAppliedPeriodicOutput decodes the gzip-compressed periodic instruction output from a
// plan secret. Returns nil if no periodic output has been written yet. Keys correspond to
// periodic instruction names.
func ReadAppliedPeriodicOutput(secret *corev1.Secret) (map[string]PeriodicInstructionOutput, error) {
	raw := secret.Data["applied-periodic-output"]
	if len(raw) == 0 {
		return nil, nil
	}
	decoded, err := decompressGzip(raw)
	if err != nil {
		return nil, fmt.Errorf("reading applied-periodic-output from %s: %w", secret.Name, err)
	}
	var out map[string]PeriodicInstructionOutput
	if err := json.Unmarshal(decoded, &out); err != nil {
		return nil, fmt.Errorf("parsing applied-periodic-output from %s: %w", secret.Name, err)
	}
	return out, nil
}

func decompressGzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
