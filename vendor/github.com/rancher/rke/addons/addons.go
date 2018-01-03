package addons

import "github.com/rancher/rke/templates"

func GetAddonsExcuteJob(addonName, nodeName, image string) (string, error) {
	jobConfig := map[string]string{
		"AddonName": addonName,
		"NodeName":  nodeName,
		"Image":     image,
	}
	return templates.CompileTemplateFromMap(templates.JobDeployerTemplate, jobConfig)
}
