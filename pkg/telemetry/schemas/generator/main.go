package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rancher/rancher/pkg/telemetry"
)

func main() {
	schema, err := telemetry.GenerateSccSchema()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate schema: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal schema: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	outputPath := "scc-RMSSubscription.json"
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write schema file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully generated and wrote JSON schema to %s\n", outputPath)
}
