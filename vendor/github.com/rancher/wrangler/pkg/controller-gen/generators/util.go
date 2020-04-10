package generators

import (
	"strings"

	"k8s.io/gengo/types"

	"k8s.io/gengo/namer"
)

const (
	GenericPackage = "github.com/rancher/wrangler/pkg/generic"
	AllSchemes     = "github.com/rancher/wrangler/pkg/schemes"
)

func groupPath(group string) string {
	g := strings.Replace(strings.Split(group, ".")[0], "-", "", -1)
	return groupPackageName(g, "")
}

func listerInformerGroupPackageName(group string, p *types.Package) string {
	parts := strings.Split(p.Path, "/")
	if len(parts) > 2 {
		p := parts[len(parts)-2]
		if p == "api" || p == "" {
			return "core"
		}
		return p
	}
	return group
}

func groupPackageName(group, groupPackageName string) string {
	if groupPackageName != "" {
		return groupPackageName
	}
	if group == "" {
		return "core"
	}
	return group
}

func upperLowercase(name string) string {
	return namer.IC(strings.ToLower(groupPath(name)))
}
