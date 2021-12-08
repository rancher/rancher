package helm

type RepoIndex struct {
	IndexFile *IndexFile `json:"indexFile" yaml:"indexFile"`
	Hash      string     `json:"hash" yaml:"hash"`
}

type IndexFile struct {
	Entries map[string]ChartVersions `json:"entries" yaml:"entries"`
}

type ChartVersions []*ChartVersion

type ChartVersion struct {
	ChartMetadata `yaml:",inline"`
	Dir           string   `json:"-" yaml:"-"`
	LocalFiles    []string `json:"-" yaml:"-"`
	URLs          []string `json:"urls" yaml:"urls"`
	Digest        string   `json:"digest,omitempty" yaml:"digest,omitempty"`
}

type ChartMetadata struct {
	Name        string   `json:"name,omitempty" yaml:"name,omitempty"`
	Sources     []string `json:"sources,omitempty" yaml:"sources,omitempty"`
	Version     string   `json:"version,omitempty" yaml:"version,omitempty"`
	KubeVersion string   `json:"kubeVersion,omitempty" yaml:"kubeVersion,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Icon        string   `json:"icon,omitempty" yaml:"icon,omitempty"`
}
