// Package crds is used for installing rancher CRDs
package crds

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"testing"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/fleet"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	fakeclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1/fake"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const (
	capiCRD           = "clusters.cluster.x-k8s.io"
	rtCRD             = "roletemplates.management.cattle.io"
	grCRD             = "globalroles.management.cattle.io"
	bootstrapFleetCRD = "clusters.fleet.cattle.io"
)

var (
	originalFs       = crdFS
	originalMigrated = MigratedResources

	//go:embed yaml_test/yaml
	validFS embed.FS

	//go:embed yaml_test/dup_yaml
	dupFs embed.FS

	//go:embed yaml_test/pod_yaml
	podFS embed.FS

	//go:embed yaml_test/bad_yaml
	badFS embed.FS

	staticCRD = &apiextv1.CustomResourceDefinition{
		TypeMeta:   v1.TypeMeta{Kind: "CustomResourceDefinition", APIVersion: "apiextensions.k8s.io/v1"},
		ObjectMeta: v1.ObjectMeta{Name: "crdName"},
		Status: apiextv1.CustomResourceDefinitionStatus{Conditions: []apiextv1.CustomResourceDefinitionCondition{
			{
				Type:    "NamesAccepted",
				Status:  "True",
				Reason:  "NoConflicts",
				Message: "no conflicts found",
			},
			{
				Type:    "Established",
				Status:  "True",
				Reason:  "InitialNamesAccepted",
				Message: "the initial names have been accepted",
			},
		}},
	}
)

func TestEnsure_MCM(t *testing.T) {
	defer resetGlobals()
	testClient := setupFakeClient()
	crdFS = validFS

	migrated := map[string]bool{rtCRD: true, capiCRD: true, grCRD: false}
	expected := []string{rtCRD, capiCRD}

	features.MCM.Set(true)

	MigratedResources = migrated
	err := EnsureRequired(context.Background(), testClient.client)
	require.NoError(t, err, "unexpected error when creating yaml")
	sort.Strings(expected)
	sort.Strings(testClient.CrdNames)
	require.Equal(t, expected, testClient.CrdNames, "unexpected CRDs created")
}

func TestEnsure_NonMCM(t *testing.T) {
	defer resetGlobals()
	testClient := setupFakeClient()
	crdFS = validFS

	features.MCM.Set(false)
	MigratedResources = map[string]bool{rtCRD: true, capiCRD: true, grCRD: false}
	expected := []string{capiCRD}

	err := EnsureRequired(context.Background(), testClient.client)
	require.NoError(t, err, "unexpected error when creating yaml")
	sort.Strings(expected)
	sort.Strings(testClient.CrdNames)
	require.Equal(t, expected, testClient.CrdNames, "unexpected CRDs created")
}

func TestEnsure_MissingCRDs(t *testing.T) {
	defer resetGlobals()
	MigratedResources = map[string]bool{"doese-not-exist": true}
	_, err := getCRDs([]string{"doese-not-exist"})
	require.Error(t, err, "expected error when CRDs could not be found")
}

func TestEnsure_DesiredFS(t *testing.T) {
	// This test is to verify that the crdFS is not accidentally changed to embed an unexpected file system.
	entries, err := crdFS.ReadDir(baseDir)
	require.NoError(t, err, "failed to read CRD FS")
	require.Len(t, entries, 1, "expected one `yaml` dir in FileSystem")
	require.Equal(t, entries[0].Name(), "yaml", "expected one `yaml` dir in FileSystem")
}

func TestEnsure_DuplicateCRDs(t *testing.T) {
	defer resetGlobals()
	crdFS = dupFs
	err := EnsureRequired(context.Background(), setupFakeClient().client)
	require.ErrorIs(t, err, errDuplicate, "expected duplicate error for redefined CRDs")
}

func TestEnsure_InvalidCRDs(t *testing.T) {
	defer resetGlobals()
	crdFS = badFS
	err := EnsureRequired(context.Background(), setupFakeClient().client)
	require.Error(t, err, "expected error when invalid YAML file is encountered")
}

func TestEnsure_NonCRDsFound(t *testing.T) {
	defer resetGlobals()
	crdFS = podFS
	err := EnsureRequired(context.Background(), setupFakeClient().client)
	require.Error(t, err, "expected error when invalid YAML file is encountered")
}

func TestEnsure_failedCreate(t *testing.T) {
	defer resetGlobals()
	testClient := setupFakeClient()
	crdFS = validFS
	MigratedResources = map[string]bool{rtCRD: true}
	testsErr := fmt.Errorf("test error")
	testClient.client.Fake.PrependReactor("create", "customresourcedefinitions", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, testsErr
	})
	features.MCM.Set(true)
	err := EnsureRequired(context.Background(), testClient.client)
	require.ErrorIs(t, err, testsErr, "expected error when creating YAML file got='%v'", err)
}

func TestEnusure_metadata(t *testing.T) {
	defer resetGlobals()
	testClient := setupFakeClient()
	crdFS = validFS

	MigratedResources = map[string]bool{rtCRD: true, capiCRD: true, bootstrapFleetCRD: true}
	expected := []string{rtCRD, capiCRD, bootstrapFleetCRD}

	features.MCM.Set(true)
	features.Fleet.Set(true)
	features.EmbeddedClusterAPI.Set(true)
	features.ProvisioningV2.Set(true)

	err := EnsureRequired(context.Background(), testClient.client)
	require.NoError(t, err, "unexpected error when creating yaml")
	sort.Strings(expected)
	sort.Strings(testClient.CrdNames)
	require.Equal(t, expected, testClient.CrdNames, "unexpected CRDs created")

	rtCRDObj, ok := testClient.CRDValues[rtCRD]
	require.True(t, ok, "%s CRD not found", rtCRD)
	require.NotNil(t, rtCRDObj.Labels, "rancher managed object missing labels")
	require.Equal(t, managerValue, rtCRDObj.Labels[k8sManagedByKey], "%s CRD missing expected managed-by label", rtCRD)

	capiCRDObj, ok := testClient.CRDValues[capiCRD]
	require.True(t, ok, "%s CRD not found", capiCRD)
	require.NotNil(t, capiCRDObj.Labels, "rancher managed object missing labels")
	require.Equal(t, managerValue, capiCRDObj.Labels[k8sManagedByKey], "%s CRD missing expected managed-by label", capiCRD)
	require.Equal(t, "true", capiCRDObj.Labels["auth.cattle.io/cluster-indexed"], "%s CRD missing expected auth label", capiCRD)

	fleetObj, ok := testClient.CRDValues[bootstrapFleetCRD]
	require.True(t, ok, "%s CRD not found", bootstrapFleetCRD)
	require.NotNil(t, fleetObj.Labels, "fleet managed object missing labels")
	require.Equal(t, "Helm", fleetObj.Labels[k8sManagedByKey], "%s CRD missing expected managed-by label", bootstrapFleetCRD)
	require.NotNil(t, fleetObj.Annotations, "fleet managed object missing annotations")
	require.Equal(t, fleet.CRDChartName, fleetObj.Annotations["meta.helm.sh/release-name"], "%s CRD missing expected annotation", bootstrapFleetCRD)
	require.Equal(t, fleet.ReleaseNamespace, fleetObj.Annotations["meta.helm.sh/release-namespace"], "%s CRD missing expected annotation", bootstrapFleetCRD)
}

func setupFakeClient() *FakeClient {
	fakeClient := &FakeClient{client: fakeclientset.NewSimpleClientset(staticCRD).ApiextensionsV1().CustomResourceDefinitions().(*fake.FakeCustomResourceDefinitions)}
	fakeClient.client.Fake.PrependReactor("create", "customresourcedefinitions", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		crd := action.(k8stesting.CreateAction).GetObject().(*apiextv1.CustomResourceDefinition)
		fakeClient.CrdNames = append(fakeClient.CrdNames, crd.Name)
		if fakeClient.CRDValues == nil {
			fakeClient.CRDValues = map[string]*apiextv1.CustomResourceDefinition{}
		}
		fakeClient.CRDValues[crd.Name] = crd
		return true, staticCRD, nil
	})
	fakeClient.client.Fake.PrependReactor("get", "customresourcedefinitions", func(k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, staticCRD, nil
	})

	return fakeClient
}

type FakeClient struct {
	client    *fake.FakeCustomResourceDefinitions
	CrdNames  []string
	CRDValues map[string]*apiextv1.CustomResourceDefinition
}

func resetGlobals() {
	crdFS = originalFs
	MigratedResources = originalMigrated
}
