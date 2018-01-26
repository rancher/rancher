package parse

import "strings"

func TemplateURLPath(path string) (string, string, string, string, bool) {
	pathSplit := strings.Split(path, ":")
	switch len(pathSplit) {
	case 2:
		catalog := pathSplit[0]
		template := pathSplit[1]
		templateSplit := strings.Split(template, "*")
		templateBase := ""
		switch len(templateSplit) {
		case 1:
			template = templateSplit[0]
		case 2:
			templateBase = templateSplit[0]
			template = templateSplit[1]
		default:
			return "", "", "", "", false
		}
		return catalog, template, templateBase, "", true
	case 3:
		catalog := pathSplit[0]
		template := pathSplit[1]
		revisionOrVersion := pathSplit[2]
		templateSplit := strings.Split(template, "*")
		templateBase := ""
		switch len(templateSplit) {
		case 1:
			template = templateSplit[0]
		case 2:
			templateBase = templateSplit[0]
			template = templateSplit[1]
		default:
			return "", "", "", "", false
		}
		return catalog, template, templateBase, revisionOrVersion, true
	default:
		return "", "", "", "", false
	}
}

func TemplatePath(path string) (string, string, bool) {
	split := strings.Split(path, "/")
	if len(split) < 2 {
		return "", "", false
	}

	base := ""
	dirSplit := strings.SplitN(split[0], "-", 2)
	if len(dirSplit) > 1 {
		base = dirSplit[0]
	}

	return base, split[1], true
}

func VersionPath(path string) (string, string, string, bool) {
	base, template, parsedCorrectly := TemplatePath(path)
	if !parsedCorrectly {
		return "", "", "", false
	}

	split := strings.Split(path, "/")
	if len(split) < 3 {
		return "", "", "", false
	}

	return base, template, split[2], true
}
