package appco

type Config struct {
	Artifacts []*Artifact `yaml:"Artifacts"`
}

type Artifact struct {
	SourceArtifact     string   `yaml:"SourceArtifact"`
	Tags               []string `yaml:"Tags"`
	TargetArtifactName string   `yaml:"TargetArtifactName,omitempty"`
	TargetRepositories []string `yaml:"TargetRepositories,omitempty"`
}
