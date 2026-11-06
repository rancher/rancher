// Command ensure-registry-cache creates the shared registry-cache pod, service, and TLS secret used by the
// provisioning tests, and waits for the pod to be running. scripts/provisioning-tests runs it before the tests
// start so the agent image can be pushed before any test needs it. The tests reuse the existing objects.
package main

import (
	"fmt"
	"os"

	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/registry"
)

func main() {
	cs, err := clients.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating clients: %v\n", err)
		os.Exit(1)
	}
	defer cs.Close()

	if _, err := registry.EnsureRegistryCache(cs); err != nil {
		fmt.Fprintf(os.Stderr, "ensuring registry-cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("registry-cache is running")
}
