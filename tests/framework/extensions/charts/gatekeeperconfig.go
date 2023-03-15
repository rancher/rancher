package charts

import "gopkg.in/yaml.v2"

func GenerateGatekeeperConfigYaml(excludedNamespaces []string, processes []string, name string, namespace string, apiVersion string, kind string) (string, error) {
	confMatch := ConfigMatch{
		{ExcludedNamespaces: excludedNamespaces,
			Processes: processes},
	}

	confSpec := ConfigSpec{
		Match: confMatch,
	}

	confMetadata := Metadata{
		Name:      name,
		Namespace: namespace,
	}

	confYaml := ConfigYaml{
		ApiVersion: apiVersion,
		Kind:       kind,
		Metadata:   confMetadata,
		Spec:       confSpec,
	}

	yamlData, err := yaml.Marshal(&confYaml)
	if err != nil {
		return "", err
	}

	yamlString := string(yamlData)

	return yamlString, err

}
