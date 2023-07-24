package configmaps

const (
	ConfigMapSteveType = "configmap"
)

type GenFieldTest struct {
	APIVersion string            `yaml:"apiVersion"`
	Data       map[string]string `yaml:"data"`
	Kind       string            `yaml:"kind"`
	Metadata   struct {
		Fields          string            `yaml:"fields"`
		Relationships   string            `yaml:"relationships"`
		State           string            `yaml:"state"`
		Labels          map[string]string `yaml:"labels"`
		Name            string            `yaml:"name"`
		Namespace       string            `yaml:"namespace"`
		ResourceVersion string            `yaml:"resourceVersion"`
		UID             string            `yaml:"uid"`
	} `yaml:"metadata"`
}
