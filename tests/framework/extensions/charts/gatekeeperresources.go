package charts

type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type ConfigYaml struct {
	ApiVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   Metadata   `yaml:"metadata"`
	Spec       ConfigSpec `yaml:"spec"`
}

type ConfigSpec struct {
	Match ConfigMatch `yaml:"match"`
}
type ConfigMatch []struct {
	ExcludedNamespaces []string `yaml:"excludedNamespaces"`
	Processes          []string `yaml:"processes"`
}

type ConstraintYaml struct {
	ApiVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   Metadata       `yaml:"metadata"`
	Spec       ConstraintSpec `yaml:"spec"`
}

type ConstraintSpec struct {
	EnforcementAction string               `yaml:"enforcementAction"`
	Match             ConstraintMatch      `yaml:"match"`
	Parameters        ConstraintParameters `yaml:"parameters"`
}
type ConstraintMatch struct {
	ExcludedNamespaces []string        `yaml:"excludedNamespaces"`
	Kinds              ConstraintKinds `yaml:"kinds"`
}

type ConstraintKinds []struct {
	ApiGroups []string `yaml:"apiGroups"`
	Kinds     []string `yaml:"kinds"`
}

type ConstraintParameters struct {
	Namespaces []string `yaml:"namespaces"`
}
