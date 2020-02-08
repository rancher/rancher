package converter

import (
	"strings"

	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

var (
	preferredGroups = map[string]string{
		"extensions": "apps",
	}
)

func AddDiscovery(client discovery.DiscoveryInterface, schemasMap map[string]*types.APISchema) error {
	logrus.Info("Refreshing all schemas")

	groups, resourceLists, err := client.ServerGroupsAndResources()
	if gd, ok := err.(*discovery.ErrGroupDiscoveryFailed); ok {
		logrus.Errorf("Failed to read API for groups %v", gd.Groups)
	} else if err != nil {
		return err
	}

	versions := indexVersions(groups)

	var errs []error
	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			errs = append(errs, err)
		}

		if err := refresh(gv, versions, resourceList, schemasMap); err != nil {
			errs = append(errs, err)
		}
	}

	return merr.NewErrors(errs...)
}

func indexVersions(groups []*metav1.APIGroup) map[string]string {
	result := map[string]string{}
	for _, group := range groups {
		result[group.Name] = group.PreferredVersion.Version
	}
	return result
}

func refresh(gv schema.GroupVersion, groupToPreferredVersion map[string]string, resources *metav1.APIResourceList, schemasMap map[string]*types.APISchema) error {
	for _, resource := range resources.APIResources {
		if strings.Contains(resource.Name, "/") {
			continue
		}

		gvk := schema.GroupVersionKind{
			Group:   gv.Group,
			Version: gv.Version,
			Kind:    resource.Kind,
		}
		gvr := gvk.GroupVersion().WithResource(resource.Name)

		logrus.Infof("APIVersion %s/%s Kind %s", gvk.Group, gvk.Version, gvk.Kind)

		schema := schemasMap[GVKToSchemaID(gvk)]
		if schema == nil {
			schema = &types.APISchema{
				Schema: &schemas.Schema{
					ID: GVKToSchemaID(gvk),
				},
			}
			attributes.SetGVK(schema, gvk)
		}

		schema.PluralName = GVRToPluralName(gvr)
		attributes.SetAPIResource(schema, resource)
		if preferredVersion := groupToPreferredVersion[gv.Group]; preferredVersion != "" && preferredVersion != gv.Version {
			attributes.SetPreferredVersion(schema, preferredVersion)
		}
		if group := preferredGroups[gv.Group]; group != "" {
			attributes.SetPreferredGroup(schema, group)
		}

		schemasMap[schema.ID] = schema
	}

	return nil
}
