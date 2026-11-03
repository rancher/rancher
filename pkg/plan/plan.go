package plan

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Parse decodes a JSON-encoded plan into a Plan struct and returns an error if the JSON is invalid.
// This is used to extract Plan Secret data into a structured format.
func Parse(raw []byte) (Plan, error) {
	var plan Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return Plan{}, fmt.Errorf("failed to parse plan: %w", err)
	}

	return plan, nil
}

// Checksum computes the sha-256 checksum of the plan bytes.
// Used for backward compatibility with orchestrators that compare applied-checksum.
func Checksum(raw []byte) string {
	h := sha256.New()
	h.Write(raw)

	return fmt.Sprintf("%x", h.Sum(nil))
}
