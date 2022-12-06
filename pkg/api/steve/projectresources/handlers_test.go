package projectresources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamic "k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	fakeauthzv1 "k8s.io/client-go/kubernetes/typed/authorization/v1/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
)

var (
	genericAllowedSARReactor = clienttesting.ReactionFunc(func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, &authzv1.SubjectAccessReview{
			Status: authzv1.SubjectAccessReviewStatus{
				Allowed: true,
			},
		}, nil
	})
	genericDeniedSARReactor = clienttesting.ReactionFunc(func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, &authzv1.SubjectAccessReview{
			Status: authzv1.SubjectAccessReviewStatus{
				Allowed: false,
			},
		}, nil
	})
)

func Test_discoveryHandler(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/apis/resources.project.cattle.io/v1alpha1", nil)

	apiResource := metav1.APIResource{
		Name:       "testresource",
		Namespaced: true,
	}
	apis := &apiResourceWatcher{
		apiResources: []metav1.APIResource{
			apiResource,
		},
	}
	h := handler{apis: apis}
	h.discoveryHandler(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := rr.Result()
	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	want := map[string]interface{}{
		"apiVersion":   "v1",
		"groupVersion": "resources.project.cattle.io/v1alpha1",
		"kind":         "APIResourceList",
		"resources":    []metav1.APIResource{apiResource},
	}
	wantJSON, _ := json.Marshal(want)
	assert.Equal(t, string(wantJSON), string(body))
}

func Test_forwarder(t *testing.T) {

	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "testgroup/testversion",
			"kind":       "TestResource",
			"metadata": map[string]interface{}{
				"namespace": "testns",
				"name":      "testname",
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		schema.GroupVersionResource{
			Group:    "testgroup",
			Version:  "testversion",
			Resource: "testresources",
		}: "TestResourceList",
	}
	clientGetter := func(_ *http.Request, _ *rest.Config, resource schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
		return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, obj).Resource(resource), nil
	}
	sarClient := fake.NewSimpleClientset().AuthorizationV1().SubjectAccessReviews()

	apis := &apiResourceWatcher{
		resourceMap: map[string]metav1.APIResource{
			"testgroup.testresources": metav1.APIResource{
				Name:       "testresources",
				Version:    "testversion",
				Group:      "testgroup",
				Namespaced: true,
			},
		},
		mapper: meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "testgroup", Version: "testversion"}}),
	}
	apis.mapper.(*meta.DefaultRESTMapper).Add(schema.GroupVersionKind{Group: "testgroup", Version: "testversion", Kind: "TestResource"}, meta.RESTScopeNamespace)

	h := handler{apis: apis, sarClient: sarClient, clientGetter: clientGetter}
	mux := mux.NewRouter()
	mux.HandleFunc("/apis/resources.project.cattle.io/v1alpha1/{resource}", h.forwarder)

	tests := []struct {
		name       string
		sarReactor clienttesting.ReactionFunc
		want       map[string]interface{}
	}{
		{
			name:       "access allowed",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "testname", "namespace": "testns"},
					},
				},
			},
		},
		{
			name:       "access denied",
			sarReactor: genericDeniedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
		},
	}
	for _, test := range tests {
		sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.PrependReactor("create", "*", test.sarReactor)
		defer sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.ClearActions()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/apis/resources.project.cattle.io/v1alpha1/testgroup.testresources", nil)
		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		resp := rr.Result()
		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
		wantJSON, _ := json.Marshal(test.want)
		assert.Equal(t, string(wantJSON), string(body))
	}
}

func Test_scopedHandler(t *testing.T) {
	scheme := runtime.NewScheme()
	objs := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns1",
					"name":      "resource1",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns2",
					"name":      "resource2",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns2b",
					"name":      "resource2",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns3",
					"name":      "resource3",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns4",
					"name":      "resource4",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "p-vwxyz",
					"name":      "resource5",
				},
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		schema.GroupVersionResource{
			Group:    "testgroup",
			Version:  "testversion",
			Resource: "testresources",
		}: "TestResourceList",
	}
	clientGetter := func(_ *http.Request, _ *rest.Config, resource schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
		return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...).Resource(resource), nil
	}
	sarClient := fake.NewSimpleClientset().AuthorizationV1().SubjectAccessReviews()

	apis := &apiResourceWatcher{
		resourceMap: map[string]metav1.APIResource{
			"testgroup.testresources": metav1.APIResource{
				Name:       "testresources",
				Version:    "testversion",
				Group:      "testgroup",
				Namespaced: true,
			},
		},
		mapper: meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "testgroup", Version: "testversion"}}),
	}
	apis.mapper.(*meta.DefaultRESTMapper).Add(schema.GroupVersionKind{Group: "testgroup", Version: "testversion", Kind: "TestResource"}, meta.RESTScopeNamespace)
	namespaceCache := &mockNamespaceCache{
		namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns1",
					Labels: map[string]string{
						"field.cattle.io/projectId":   "p-abcde",
						"kubernetes.io/metadata.name": "ns1",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns2",
					Labels: map[string]string{
						"field.cattle.io/projectId":   "p-jklmn",
						"kubernetes.io/metadata.name": "ns2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns2b",
					Labels: map[string]string{
						"field.cattle.io/projectId":   "p-jklmn",
						"kubernetes.io/metadata.name": "ns2b",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns3",
					Labels: map[string]string{
						"field.cattle.io/projectId":   "p-vwxyz",
						"kubernetes.io/metadata.name": "ns3",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns4",
					// no projectId label == orphan namespace
					Labels: map[string]string{
						"kubernetes.io/metadata.name": "ns4",
					},
				},
			},
			// project namespaces
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-abcde",
					Labels: map[string]string{
						"cattle.io/parent":            "true",
						"kubernetes.io/metadata.name": "p-abcde",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-jklmn",
					Labels: map[string]string{
						"cattle.io/parent":            "true",
						"kubernetes.io/metadata.name": "p-jklmn",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					// namespace with project name
					Name: "p-vwxyz",
					Labels: map[string]string{
						"cattle.io/parent":            "true",
						"kubernetes.io/metadata.name": "p-vwxyz",
					},
				},
			},
		},
	}

	h := handler{apis: apis, clientGetter: clientGetter, sarClient: sarClient, namespaceCache: namespaceCache}
	mux := mux.NewRouter()
	mux.HandleFunc("/namespaces/{project}/{resource}", h.scopedHandler).Queries(queryKey, queryValue)
	mux.HandleFunc("/namespaces/{project}/{resource}", h.scopedHandler)

	tests := []struct {
		name       string
		request    string
		namespaces []corev1.Namespace
		want       map[string]interface{}
		wantStatus int
		sarReactor clienttesting.ReactionFunc
	}{
		{
			name:       "no selector",
			request:    "/namespaces/p-abcde/testgroup.testresources",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "one matching project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "multi matching projects",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde,p-jklmn,p-vwxyz",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "one nonmatching project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=p-vwxyz",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "multi nonmatching projects",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=p-jklmn,p-vwxyz",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "namespaces in project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=ns1",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "namespaces not in project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=ns2,ns3,ns4",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "some namespaces in project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=ns1,ns2,ns3,ns4",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "assorted projects and namespaces",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde,p-vwxyz,ns1,ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not single matching project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=p-abcde",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not multi matching project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=p-abcde,p-vwxyz",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not single non matching project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=p-vwxyz",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not multi non matching project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=p-jklmn,p-vwxyz",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not single namespace in project",
			request:    "/namespaces/p-jklmn/testgroup.testresources?fieldSelector=projectsornamespaces!=ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2b"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not single namespace not in project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not multi namespace in project",
			request:    "/namespaces/p-jklmn/testgroup.testresources?fieldSelector=projectsornamespaces!=ns1,ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2b"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not multi namespace not in project",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not combo matching project and non matching namespace",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=p-abcde,ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not combo non matching project and non matching namespace",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=p-jklmn,ns2,ns3",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not combo non matching project and matching namespace",
			request:    "/namespaces/p-abcde/testgroup.testresources?fieldSelector=projectsornamespaces!=p-jklmn,ns1",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "access denied to some namespaces",
			request: "/namespaces/p-jklmn/testgroup.testresources",
			sarReactor: clienttesting.ReactionFunc(func(action clienttesting.Action) (bool, runtime.Object, error) {
				result := &authzv1.SubjectAccessReview{
					Status: authzv1.SubjectAccessReviewStatus{},
				}
				if action.(clienttesting.CreateActionImpl).GetObject().(*authzv1.SubjectAccessReview).Spec.ResourceAttributes.Namespace == "ns2b" {
					result.Status.Allowed = true
				}
				return true, result, nil
			}),
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2b"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "access denied to all namespaces",
			request:    "/namespaces/p-jklmn/testgroup.testresources",
			sarReactor: genericDeniedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.PrependReactor("create", "*", test.sarReactor)
			defer sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.ClearActions()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", test.request, nil)
			mux.ServeHTTP(rr, req)
			assert.Equal(t, test.wantStatus, rr.Code)
			resp := rr.Result()
			body, err := io.ReadAll(resp.Body)
			assert.Nil(t, err)
			if test.want != nil {
				wantJSON, _ := json.Marshal(test.want)
				assert.Equal(t, string(wantJSON), string(body))
			}
		})
	}
}

func Test_globalHandler(t *testing.T) {
	scheme := runtime.NewScheme()
	objs := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns1",
					"name":      "resource1",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns2",
					"name":      "resource2",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns3",
					"name":      "resource3",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns4",
					"name":      "resource4",
				},
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		schema.GroupVersionResource{
			Group:    "testgroup",
			Version:  "testversion",
			Resource: "testresources",
		}: "TestResourceList",
	}
	clientGetter := func(_ *http.Request, _ *rest.Config, resource schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
		return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...).Resource(resource), nil
	}
	sarClient := fake.NewSimpleClientset().AuthorizationV1().SubjectAccessReviews()

	apis := &apiResourceWatcher{
		resourceMap: map[string]metav1.APIResource{
			"testgroup.testresources": metav1.APIResource{
				Name:       "testresources",
				Version:    "testversion",
				Group:      "testgroup",
				Namespaced: true,
			},
		},
		mapper: meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "testgroup", Version: "testversion"}}),
	}
	apis.mapper.(*meta.DefaultRESTMapper).Add(schema.GroupVersionKind{Group: "testgroup", Version: "testversion", Kind: "TestResource"}, meta.RESTScopeNamespace)
	namespaceCache := &mockNamespaceCache{
		namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns1",
					Labels: map[string]string{
						"field.cattle.io/projectId":   "p-abcde",
						"kubernetes.io/metadata.name": "ns1",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns2",
					Labels: map[string]string{
						"field.cattle.io/projectId":   "p-jklmn",
						"kubernetes.io/metadata.name": "ns2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns3",
					Labels: map[string]string{
						"field.cattle.io/projectId":   "p-vwxyz",
						"kubernetes.io/metadata.name": "ns3",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns4",
					// no projectId label == orphan namespace
					Labels: map[string]string{
						"kubernetes.io/metadata.name": "ns4",
					},
				},
			},
			// project namespaces
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-abcde",
					Labels: map[string]string{
						"cattle.io/parent":            "true",
						"kubernetes.io/metadata.name": "p-abcde",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-jklmn",
					Labels: map[string]string{
						"cattle.io/parent":            "true",
						"kubernetes.io/metadata.name": "p-jklmn",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					// namespace with project name
					Name: "p-vwxyz",
					Labels: map[string]string{
						"cattle.io/parent":            "true",
						"kubernetes.io/metadata.name": "p-vwxyz",
					},
				},
			},
		},
	}

	mux := mux.NewRouter()
	h := handler{apis: apis, clientGetter: clientGetter, sarClient: sarClient, namespaceCache: namespaceCache}
	mux.HandleFunc("/{resource}", h.globalHandler).Queries(queryKey, queryValue)

	tests := []struct {
		name       string
		request    string
		sarReactor clienttesting.ReactionFunc
		namespaces []corev1.Namespace
		want       map[string]interface{}
		wantStatus int
	}{
		{
			name:       "no selector",
			request:    "/testgroup.testresources",
			sarReactor: genericAllowedSARReactor,
			want:       nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "one project",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata": map[string]interface{}{
					"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "multi project",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde,p-jklmn,p-vwxyz",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource3", "namespace": "ns3"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "one namespace",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces=ns1",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "multi namespace",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces=ns1,ns3,ns4",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource3", "namespace": "ns3"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "assorted projects and namespaces",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde,ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not single namespace",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces!=ns1",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource3", "namespace": "ns3"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not single project",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces!=p-abcde",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource3", "namespace": "ns3"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not multi namespace",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces!=ns1,ns2",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource3", "namespace": "ns3"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not multi project",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces!=p-abcde,p-jklmn",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource3", "namespace": "ns3"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not combo project and namespace",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces!=p-abcde,ns3",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource2", "namespace": "ns2"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "access denied to some namespaces",
			request: "/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde,p-jklmn,p-vwxyz",
			sarReactor: clienttesting.ReactionFunc(func(action clienttesting.Action) (bool, runtime.Object, error) {
				result := &authzv1.SubjectAccessReview{
					Status: authzv1.SubjectAccessReviewStatus{},
				}
				namespace := action.(clienttesting.CreateActionImpl).GetObject().(*authzv1.SubjectAccessReview).Spec.ResourceAttributes.Namespace
				if namespace == "ns1" || namespace == "ns3" {
					result.Status.Allowed = true
				}
				return true, result, nil
			}),
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource1", "namespace": "ns1"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource3", "namespace": "ns3"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "access denied to all namespaces",
			request:    "/testgroup.testresources?fieldSelector=projectsornamespaces=p-abcde,p-jklmn,p-vwxyz",
			sarReactor: genericDeniedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.PrependReactor("create", "*", test.sarReactor)
			defer sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.ClearActions()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", test.request, nil)
			mux.ServeHTTP(rr, req)
			assert.Equal(t, test.wantStatus, rr.Code)
			resp := rr.Result()
			body, err := io.ReadAll(resp.Body)
			assert.Nil(t, err)
			if test.want != nil {
				wantJSON, _ := json.Marshal(test.want)
				assert.Equal(t, string(wantJSON), string(body))
			}
		})
	}
}

func Test_unscopedHandler(t *testing.T) {
	scheme := runtime.NewScheme()
	objs := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns1",
					"name":      "resource1",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns2",
					"name":      "resource2",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns4",
					"name":      "resource4",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "ns5",
					"name":      "resource5",
				},
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		schema.GroupVersionResource{
			Group:    "testgroup",
			Version:  "testversion",
			Resource: "testresources",
		}: "TestResourceList",
	}
	clientGetter := func(_ *http.Request, _ *rest.Config, resource schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
		return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...).Resource(resource), nil
	}
	sarClient := fake.NewSimpleClientset().AuthorizationV1().SubjectAccessReviews()

	apis := &apiResourceWatcher{
		resourceMap: map[string]metav1.APIResource{
			"testgroup.testresources": metav1.APIResource{
				Name:       "testresources",
				Version:    "testversion",
				Group:      "testgroup",
				Namespaced: true,
			},
		},
		mapper: meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "testgroup", Version: "testversion"}}),
	}
	apis.mapper.(*meta.DefaultRESTMapper).Add(schema.GroupVersionKind{Group: "testgroup", Version: "testversion", Kind: "TestResource"}, meta.RESTScopeNamespace)

	namespaceCache := &mockNamespaceCache{
		namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns4",
					// no projectId labels == orphan namespace
					Labels: map[string]string{
						"kubernetes.io/metadata.name": "ns4",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns5",
					// no projectId labels == orphan namespace
					Labels: map[string]string{
						"kubernetes.io/metadata.name": "ns5",
					},
				},
			},
		},
	}

	h := handler{apis: apis, clientGetter: clientGetter, sarClient: sarClient, namespaceCache: namespaceCache}
	mux := mux.NewRouter()
	mux.HandleFunc("/namespaces/cattle-unscoped/{resource}", h.unscopedHandler).Queries(queryKey, queryValue)
	mux.HandleFunc("/namespaces/cattle-unscoped/{resource}", h.unscopedHandler)

	partialDenyReactor := clienttesting.ReactionFunc(func(action clienttesting.Action) (bool, runtime.Object, error) {
		result := &authzv1.SubjectAccessReview{
			Status: authzv1.SubjectAccessReviewStatus{},
		}
		if action.(clienttesting.CreateActionImpl).GetObject().(*authzv1.SubjectAccessReview).Spec.ResourceAttributes.Namespace == "ns5" {
			result.Status.Allowed = true
		}
		return true, result, nil
	})

	tests := []struct {
		name       string
		request    string
		namespaces []corev1.Namespace
		sarReactor clienttesting.ReactionFunc
		want       map[string]interface{}
		wantStatus int
	}{
		{
			name:       "no selector",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource5", "namespace": "ns5"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "select by existing namespace",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources?fieldSelector=projectsornamespaces=ns4",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "select by nonexisting namespace",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources?fieldSelector=projectsornamespaces=ns10",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not existing namespace",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources?fieldSelector=projectsornamespaces!=ns4",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource5", "namespace": "ns5"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not nonexisting namespace",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources?fieldSelector=projectsornamespaces!=ns10",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource5", "namespace": "ns5"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not namespace in project",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources?fieldSelector=projectsornamespaces!=n1",
			sarReactor: genericAllowedSARReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource4", "namespace": "ns4"},
					},
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource5", "namespace": "ns5"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "access to only one namespace",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources",
			sarReactor: partialDenyReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource5", "namespace": "ns5"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "select by accessible namespace",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources?fieldSelector=projectsornamespaces=ns5",
			sarReactor: partialDenyReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items": []map[string]interface{}{
					{
						"apiVersion": "testgroup/testversion",
						"kind":       "TestResource",
						"metadata":   map[string]interface{}{"name": "resource5", "namespace": "ns5"},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "select by inaccessible namespace",
			request:    "/namespaces/cattle-unscoped/testgroup.testresources?fieldSelector=projectsornamespaces=ns4",
			sarReactor: partialDenyReactor,
			want: map[string]interface{}{
				"apiVersion": "testgroup/testversion",
				"kind":       "TestResourceList",
				"metadata":   map[string]interface{}{"resourceVersion": ""},
				"items":      []map[string]interface{}{},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.PrependReactor("create", "*", test.sarReactor)
			defer sarClient.(*fakeauthzv1.FakeSubjectAccessReviews).Fake.ClearActions()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", test.request, nil)
			mux.ServeHTTP(rr, req)
			assert.Equal(t, test.wantStatus, rr.Code)
			resp := rr.Result()
			body, err := io.ReadAll(resp.Body)
			assert.Nil(t, err)
			if test.want != nil {
				wantJSON, _ := json.Marshal(test.want)
				assert.Equal(t, string(wantJSON), string(body))
			}
		})
	}
}

func Test_gvrFromVars(t *testing.T) {
	apis := apiResourceWatcher{
		resourceMap: map[string]metav1.APIResource{
			"pods": metav1.APIResource{
				Name:       "pods",
				Version:    "v1",
				Group:      "",
				Namespaced: true,
			},
			"apps.deployments": metav1.APIResource{
				Name:       "deployments",
				Version:    "v1",
				Group:      "apps",
				Namespaced: true,
			},
			"rbac.authorization.k8s.io.roles": metav1.APIResource{
				Name:       "roles",
				Version:    "v1",
				Group:      "rbac.authorization.k8s.io",
				Namespaced: true,
			},
		},
	}
	h := &handler{apis: &apis}

	tests := []struct {
		name    string
		vars    map[string]string
		want    schema.GroupVersionResource
		wantErr error
	}{
		{
			name:    "invalid resource",
			vars:    map[string]string{"resource": "notaresources"},
			wantErr: fmt.Errorf("could not find resource notaresources"),
		},
		{
			name:    "invalid group and resource",
			vars:    map[string]string{"resource": "notagroup.notaresources"},
			wantErr: fmt.Errorf("could not find resource notagroup.notaresources"),
		},
		{
			name: "/api/v1/-style resource",
			vars: map[string]string{"resource": "pods"},
			want: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
		{
			name: "/apis/-style resource",
			vars: map[string]string{"resource": "apps.deployments"},
			want: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
		},
		{
			name: "multi-segment group resource",
			vars: map[string]string{"resource": "rbac.authorization.k8s.io.roles"},
			want: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "roles",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, gotErr := h.gvrFromVars(test.vars)
			assert.Equal(t, test.wantErr, gotErr)
			assert.Equal(t, test.want, got)
		})
	}
}

type mockNamespaceCache struct {
	namespaces []*corev1.Namespace
}

func (m *mockNamespaceCache) Get(name string) (*corev1.Namespace, error) {
	for _, n := range m.namespaces {
		if n.Name == name {
			return n, nil
		}
	}
	return nil, &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Code: 404,
		},
	}
}

func (m *mockNamespaceCache) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	result := []*corev1.Namespace{}
	for _, n := range m.namespaces {
		if selector.Matches(labels.Set(n.ObjectMeta.Labels)) {
			result = append(result, n)
		}
	}
	return result, nil
}
func (m *mockNamespaceCache) AddIndexer(indexName string, indexer corecontrollers.NamespaceIndexer) {
	panic("not implemented")
}

func (m *mockNamespaceCache) GetByIndex(indexName, key string) ([]*corev1.Namespace, error) {
	panic("not implemented")
}
