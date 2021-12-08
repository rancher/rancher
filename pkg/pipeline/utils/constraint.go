package utils

import (
	"path/filepath"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
)

func MatchAll(cs *v32.Constraints, execution *v3.PipelineExecution) bool {
	if cs == nil {
		return true
	}

	m := GetEnvVarMap(execution)

	return Match(cs.Branch, m[EnvGitBranch]) &&
		Match(cs.Event, m[EnvEvent])
}

func Match(c *v32.Constraint, v string) bool {
	if c == nil {
		return true
	}

	if Excludes(c, v) {
		return false
	}
	if Includes(c, v) {
		return true
	}
	if len(c.Include) == 0 {
		return true
	}
	return false
}

// Includes returns true if the string matches the include patterns.
func Includes(c *v32.Constraint, v string) bool {
	for _, pattern := range c.Include {
		if ok, _ := filepath.Match(pattern, v); ok {
			return true
		}
	}
	return false
}

// Excludes returns true if the string matches the exclude patterns.
func Excludes(c *v32.Constraint, v string) bool {
	for _, pattern := range c.Exclude {
		if ok, _ := filepath.Match(pattern, v); ok {
			return true
		}
	}
	return false
}
