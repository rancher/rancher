package main

import (
	"context"
	"os"
)

// preStart is meant as a replacement for entrypoint logic.
// These actions were previously executed as part of the agent image's entrypoint script, before actually executing the agent binary
// The logic has been migrated from that shell script to native Go
func preStart(ctx context.Context) error {
	if os.Getenv("CATTLE_ENTRYPOINT_BYPASS") == "true" {
		return nil
	}
	return nil
}
