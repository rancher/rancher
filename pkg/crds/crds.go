// Package crds is used for installing rancher CRDs.
package crds

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clientv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	baseDir = "."

	// readyDuration time to wait for CRDs to be ready.
	readyDuration = time.Minute * 1

	k8sManagedByKey = "app.kubernetes.io/managed-by"
	managerValue    = "rancher"
	crdKind         = "CustomResourceDefinition"
)

var (
	//go:embed yaml
	crdFS embed.FS

	errDuplicate = fmt.Errorf("duplicate CRD")
)

// EnsureRequired will ensure all required CRDs needed by Rancher based on the currently enabled features are installed and up to date.
func EnsureRequired(ctx context.Context, crdClient clientv1.CustomResourceDefinitionInterface) error {
	return Ensure(ctx, crdClient, RequiredCRDs())
}

// Ensure will ensure all given CRD names are installed and up to date.
// Ensure looks for matching crd names inside the embedded directory 'yaml'.
func Ensure(ctx context.Context, crdClient clientv1.CustomResourceDefinitionInterface, crdNames []string) error {
	toCreateCRDs, err := getCRDs(crdNames)
	if err != nil {
		return fmt.Errorf("failed to get CRDs: %w", err)
	}

	ownedByRancher, err := labels.NewRequirement(k8sManagedByKey, selection.Equals, []string{managerValue})
	if err != nil {
		return fmt.Errorf("failed to create crd label selector: %w", err)
	}
	selector := labels.NewSelector().Add(*ownedByRancher)

	err = crd.BatchCreateCRDs(ctx, crdClient, selector, readyDuration, toCreateCRDs)
	if err != nil {
		return fmt.Errorf("failed to create CRDs: %w", err)
	}

	return nil
}

// getCRDs finds all embedded CRDs that match the given list of CRDs. If a CRD is specified and not found an error is returned.
func getCRDs(crdNames []string) ([]*apiextv1.CustomResourceDefinition, error) {
	allCRDs, err := crdsFromDir(baseDir)
	if err != nil {
		return nil, err
	}

	capiMap := toMap(CAPICRDs())
	bootstrapFleetMap := toMap(bootstrapFleet())

	retCRDs := []*apiextv1.CustomResourceDefinition{}

	for _, crdName := range crdNames {
		// only return CRDs that are specified and have been migrated to the new CRD generation flow
		if !MigratedResources[crdName] {
			continue
		}

		crd, found := allCRDs[crdName]
		if !found {
			return nil, fmt.Errorf("CRD yaml '%s' not found in embedded file system", crdName)
		}

		if crd.Labels == nil {
			crd.Labels = map[string]string{}
		}

		// dynamically add labels to capi crds since we do not generate them.
		if capiMap[crd.Name] {
			crd.Labels["auth.cattle.io/cluster-indexed"] = "true"
		}

		// Add managed by label
		if bootstrapFleetMap[crd.Name] {
			// Ensure labels/annotations are set so that helm will manage this
			crd.Labels[k8sManagedByKey] = "Helm"
			if crd.Annotations == nil {
				crd.Annotations = map[string]string{}
			}
			crd.Annotations["meta.helm.sh/release-name"] = fleet.CRDChartName
			crd.Annotations["meta.helm.sh/release-namespace"] = fleet.ReleaseNamespace
		} else {
			// since this CRD is installed by rancher add the manged by label
			crd.Labels[k8sManagedByKey] = managerValue
		}

		retCRDs = append(retCRDs, crd)
	}

	return retCRDs, nil
}

// crdsFromDir recursively traverses the embedded yaml directory and find all CRD yamls.
func crdsFromDir(dirName string) (map[string]*apiextv1.CustomResourceDefinition, error) {
	// read all entries in the embedded directory
	crdFiles, err := crdFS.ReadDir(dirName)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded dir '%s': %w", dirName, err)
	}

	allCRDs := map[string]*apiextv1.CustomResourceDefinition{}
	for _, dirEntry := range crdFiles {
		fullPath := filepath.Join(dirName, dirEntry.Name())
		if dirEntry.IsDir() {
			// if the entry is the dir recurse into that folder to get all crds
			subCRDs, err := crdsFromDir(fullPath)
			if err != nil {
				return nil, err
			}
			for k, v := range subCRDs {
				if _, ok := allCRDs[k]; ok {
					return nil, fmt.Errorf("%w for '%s", errDuplicate, k)
				}
				allCRDs[k] = v
			}
			continue
		}

		// read the file and convert it to a crd object
		file, err := crdFS.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open embedded file '%s': %w", fullPath, err)
		}
		crdObjs, err := yaml.UnmarshalWithJSONDecoder[*apiextv1.CustomResourceDefinition](file)
		if err != nil {
			return nil, fmt.Errorf("failed to convert embedded file '%s' to yaml: %w", fullPath, err)
		}
		for _, crdObj := range crdObjs {
			if crdObj.Kind != crdKind {
				// if the yaml is not a CRD return an error
				return nil, fmt.Errorf("decoded object is not '%s' instead found Kind='%s'", crdKind, crdObj.Kind)
			}
			if _, ok := allCRDs[crdObj.Name]; ok {
				return nil, fmt.Errorf("%w for '%s", errDuplicate, crdObj.Name)
			}
			allCRDs[crdObj.Name] = crdObj
		}
	}
	return allCRDs, nil
}

// toMap converts a list of slice to a map.
func toMap(strings []string) map[string]bool {
	retMap := make(map[string]bool, len(strings))
	for _, name := range strings {
		retMap[name] = true
	}
	return retMap
}
