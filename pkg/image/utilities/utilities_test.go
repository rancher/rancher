package utilities

import (
	"testing"
)

func TestCheckImage(t *testing.T) {
	imageListAndErrorExpectations := map[string]bool{
		"weaveworks/npc:latest":         false,
		"noiro/test:latest":             false,
		"registry.suse.com/test:latest": true,
		"rancher/aks-operator:latest":   false,
		"google/gke-operator:latest":    true, // not from 'rancher/' or whitelisted
		"rancher/gke-operator-:latest":  true, // trailing '-' in image name
		"rancher/test":                  true, // missing tag
		"rancher/test:":                 true, // empty tag
	}

	for k, v := range imageListAndErrorExpectations {
		err := checkImage(k)
		if err != nil && !v {
			t.Logf("did not expect error when checking image %s", k)
			t.Fail()
		}
		if err == nil && v {
			t.Logf("expected error when checking image %s", k)
			t.Fail()
		}
	}
}
