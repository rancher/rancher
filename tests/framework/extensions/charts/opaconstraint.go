package charts

import (
	"gopkg.in/yaml.v2"
)

func GenerateGatekeeperConstraintYaml(apiGroups []string, excludedNamespaces []string, kinds []string, name string, namespaces []string, enforcementAction string, apiVersion string, kind string) (string, error) {

	nSKinds := ConstraintKinds{
		{ApiGroups: apiGroups},
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
		ApiVersion: apiVersion,
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
