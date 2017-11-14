package helm

import (
	"time"
)

type RepoIndex struct {
	URL       string     `json:"url" yaml:"url"`
	CertFile  string     `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile   string     `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	CAFile    string     `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	IndexFile *IndexFile `json:"indexFile" yaml:"indexFile"`
	Hash      string     `json:"hash" yaml:"hash"`
}

type IndexFile struct {
	APIVersion string                   `json:"apiVersion" yaml:"apiVersion"`
	Generated  time.Time                `json:"generated" yaml:"generated"`
	Entries    map[string]ChartVersions `json:"entries" yaml:"entries"`
	PublicKeys []string                 `json:"publicKeys,omitempty" yaml:"publicKeys,omitempty"`
}

type ChartVersions []*ChartVersion

type ChartVersion struct {
	ChartMetadata `yaml:",inline"`
	URLs          []string  `json:"urls" yaml:"urls"`
	Created       time.Time `json:"created,omitempty" yaml:"created,omitempty"`
	Removed       bool      `json:"removed,omitempty" yaml:"removed,omitempty"`
	Digest        string    `json:"digest,omitempty" yaml:"digest,omitempty"`
}

type ChartMetadata struct {
	Name        string        `json:"name,omitempty" yaml:"name,omitempty"`
	Home        string        `json:"home,omitempty" yaml:"home,omitempty"`
	Sources     []string      `json:"sources,omitempty" yaml:"sources,omitempty"`
	Version     string        `json:"version,omitempty" yaml:"version,omitempty"`
	Description string        `json:"description,omitempty" yaml:"description,omitempty"`
	Keywords    []string      `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Maintainers []*Maintainer `json:"maintainers,omitempty" yaml:"maintainers,omitempty"`
	Engine      string        `json:"engine,omitempty" yaml:"engine,omitempty"`
	Icon        string        `json:"icon,omitempty" yaml:"icon,omitempty"`
	APIVersion  string        `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Condition   string        `json:"condition,omitempty" yaml:"condition,omitempty"`
	Tags        string        `json:"tags,omitempty" yaml:"tags,omitempty"`
	AppVersion  string        `json:"appVersion,omitempty" yaml:"appVersion,omitempty"`
	Deprecated  bool          `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
}

type Maintainer struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}
