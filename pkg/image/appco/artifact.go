package appco

import "strings"

func (a *Artifact) URLs() []string {
	repo, name := normalizeSource(a.SourceArtifact)
	if repo == "" || name == "" {
		return []string{}
	}

	if a.TargetArtifactName != "" {
		name = a.TargetArtifactName
	}

	out := make([]string, 0, len(a.Tags))
	for _, tag := range a.Tags {
		out = append(out, repo+"/"+name+":"+tag)
	}

	return out
}

func normalizeSource(src string) (repo, name string) {
	if strings.HasPrefix(src, "dp.apps.rancher.io/charts/") {
		name = strings.TrimPrefix(src, "dp.apps.rancher.io/charts/")
		if !strings.HasPrefix(name, "appco-") {
			name = "appco-" + name
		}
		return "rancher/charts", name
	} else if strings.HasPrefix(src, "dp.apps.rancher.io/containers/") {
		name = strings.TrimPrefix(src, "dp.apps.rancher.io/containers/")
		if !strings.HasPrefix(name, "appco-") {
			name = "appco-" + name
		}
		return "rancher", name
	}

	return "", ""
}
