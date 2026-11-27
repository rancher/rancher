package userpreferences

import (
	"net/http/httptest"
	"testing"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/namespace"
	steveclient "github.com/rancher/steve/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

// fakeClientGetter embeds the real interface and only overrides AdminClient.
type fakeClientGetter struct {
	steveclient.ClientGetter
	client dynamic.ResourceInterface
}

func (f *fakeClientGetter) AdminClient(_ *types.APIRequest, _ *types.APISchema, _ string, _ rest.WarningHandler) (dynamic.ResourceInterface, error) {
	return f.client, nil
}

func TestCreateNamespace(t *testing.T) {
	namespaces := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()).
		Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"})
	store := &rancherPrefStore{cg: &fakeClientGetter{client: namespaces}}
	apiOp := &types.APIRequest{
		Request: httptest.NewRequest("PUT", "/", nil),
		Schemas: types.EmptyAPISchemas(),
	}

	require.NoError(t, store.createNamespace(apiOp, "u-test"))

	ns, err := namespaces.Get(apiOp.Context(), "u-test", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "true", ns.GetAnnotations()[namespace.SystemNamespaceAnnotation])
}
