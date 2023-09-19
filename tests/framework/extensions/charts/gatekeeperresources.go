package charts

// These are the structs used to generate the yaml for OPA Gatekeeper Constraints and Configs
// Constraints are what Gatekeeper uses to describe and enforce specific policies in a cluster. More on Constraints: https://open-policy-agent.github.io/gatekeeper/website/docs/howto
// Configs define clusterwide rules for Gatekeeper. More on Configs: https://open-policy-agent.github.io/gatekeeper/website/docs/exempt-namespaces

// Metadata struct can be shared by Configs and Constraints
type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// ConfigYaml all the structs that make up a gatekeeper Config are nested in this struct, Configs and Constraints are similar K8s objects, but require different Specs
type ConfigYaml struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   Metadata   `yaml:"metadata"`
	Spec       ConfigSpec `yaml:"spec"`
}

// ConfigSpec spec field for ConfigYaml, it contains different fields than ConstraintSpec
type ConfigSpec struct {
	Match ConfigMatch `yaml:"match"`
}

// ConfigMatch match field for ConfigYaml, it contains different fields than ConstraintMatch
type ConfigMatch []struct {
	ExcludedNamespaces []string `yaml:"excludedNamespaces"`
	Processes          []string `yaml:"processes"`
}

// ConstraintYaml All the structs that make up a gatekeeper Constraint are nested in this struct, Configs and Constraints are similar K8s objects, but require different Specs
type ConstraintYaml struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   Metadata       `yaml:"metadata"`
	Spec       ConstraintSpec `yaml:"spec"`
}

// ConstraintSpec spec field for ConstraintYaml, it contains different fields than ConfigSpec
type ConstraintSpec struct {
	EnforcementAction string               `yaml:"enforcementAction"`
	Match             ConstraintMatch      `yaml:"match"`
	Parameters        ConstraintParameters `yaml:"parameters"`
}

// ConstraintMatch match field for ConstraintYaml, it contains different fields than ConfigMatch
type ConstraintMatch struct {
	ExcludedNamespaces []string        `yaml:"excludedNamespaces"`
	Kinds              ConstraintKinds `yaml:"kinds"`
}

// ConstraintKinds field
type ConstraintKinds []struct {
	APIGroups []string `yaml:"apiGroups"`
	Kinds     []string `yaml:"kinds"`
}

// ConstraintParameters field
type ConstraintParameters struct {
	Namespaces []string `yaml:"namespaces"`
}
