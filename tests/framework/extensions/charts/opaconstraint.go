package charts

import (
	"gopkg.in/yaml.v2"
)

// GenerateGatekeeperConstraintYaml takes inputs to generate the yaml for an OPA Gatekeeper Constraint from an OPA Constraint Template. This can be used to dynamically generate OPA Constraints based on test data
func GenerateGatekeeperConstraintYaml(apiGroups []string, excludedNamespaces []string, kinds []string, name string, namespaces []string, enforcementAction string, apiVersion string, kind string) (string, error) {

	nSKinds := ConstraintKinds{
		{APIGroups: apiGroups},
		{Kinds: kinds},
	}

	nSMetadata := Metadata{
		Name: name,
	}

	nSParameters := ConstraintParameters{
		Namespaces: namespaces,
	}

	nSMatch := ConstraintMatch{
		ExcludedNamespaces: excludedNamespaces,
		Kinds:              nSKinds,
	}

	nSSpec := ConstraintSpec{
		EnforcementAction: enforcementAction,
		Match:             nSMatch,
		Parameters:        nSParameters,
	}

	allowedNamespaces := ConstraintYaml{
		APIVersion: apiVersion,
		Kind:       kind,
		Metadata:   nSMetadata,
		Spec:       nSSpec,
	}

	yamlData, err := yaml.Marshal(&allowedNamespaces)
	if err != nil {
		return "", err
	}

	yamlString := string(yamlData)

	return yamlString, err
}
