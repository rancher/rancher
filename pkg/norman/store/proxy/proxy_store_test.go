package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
)

func TestGetDeletionOptions(t *testing.T) {
	req, err := http.NewRequest("DELETE", "https://test.url/api", nil)
	assert.Empty(t, err)
	prop := metav1.DeletePropagationBackground
	expected := &metav1.DeleteOptions{
		PropagationPolicy: &prop,
	}
	options, err := getDeleteOption(req)
	assert.Empty(t, err)
	assert.Equal(t, options, expected, "unexpected deletion options for empty query")

	req.URL.RawQuery = "gracePeriodSeconds=0"
	period := int64(0)
	expected = &metav1.DeleteOptions{
		PropagationPolicy:  &prop,
		GracePeriodSeconds: &period,
	}
	options, err = getDeleteOption(req)
	assert.Empty(t, err)
	assert.Equal(t, options, expected, "unexpected deletion options for query 'gracePeriodSeconds=0'")
}

func TestList(t *testing.T) {

	var data = v1.ConfigMapList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMapList",
			APIVersion: "v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion:    "v1",
			RemainingItemCount: new(int64),
		},
		Items: []v1.ConfigMap{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "default",
				},
				Immutable: new(bool),
				Data: map[string]string{
					"a": "av",
					"b": "bv",
					"c": "cv",
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2",
					Namespace: "default",
				},
				Immutable: new(bool),
				Data: map[string]string{
					"a2": "av",
					"b2": "bv",
					"c2": "cv",
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "default",
				},
				Immutable: new(bool),
				Data: map[string]string{
					"a3": "av",
					"b3": "bv",
					"c3": "cv",
				},
			},
		},
	}

	clientGetter := mockClientGetter{
		&fake.RESTClient{
			NegotiatedSerializer: serializer.NewCodecFactory(runtime.NewScheme()),
		},
	}

	typer := runtime.NewScheme()

	var sut = &Store{
		Mutex:          sync.Mutex{},
		clientGetter:   &clientGetter,
		group:          "",
		version:        "v1",
		kind:           "ConfigMap",
		resourcePlural: "configmaps",
		typer:          typer,
	}

	schema := types.Schema{
		Mapper: types.Mappers{},
	}

	req, _ := http.NewRequest(http.MethodGet, "", nil)
	apiContext := types.APIContext{
		Request:       req,
		AccessControl: &authorization.AllAccess{},
	}

	// no results
	{
		body := data
		body.Items = nil
		var fakeResponse bytes.Buffer
		_ = json.NewEncoder(&fakeResponse).Encode(body)
		clientGetter.Resp = &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(&fakeResponse),
		}

		res, err := sut.List(&apiContext, &schema, &types.QueryOptions{})

		assert.NoError(t, err)
		assert.IsType(t, []map[string]interface{}{}, res)
		assert.Len(t, res, 0)
	}

	// generic type
	{
		body := data
		var fakeResponse bytes.Buffer
		_ = json.NewEncoder(&fakeResponse).Encode(body)
		clientGetter.Resp = &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(&fakeResponse),
		}

		res, err := sut.List(&apiContext, &schema, &types.QueryOptions{})

		assert.NoError(t, err)
		assert.IsType(t, []map[string]interface{}{}, res)
		assert.Len(t, res, 3)
	}

	_ = v1.SchemeBuilder.AddToScheme(typer)

	// specific type
	{
		body := data
		var fakeResponse bytes.Buffer
		_ = json.NewEncoder(&fakeResponse).Encode(body)
		clientGetter.Resp = &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(&fakeResponse),
		}

		res, err := sut.List(&apiContext, &schema, &types.QueryOptions{})

		assert.NoError(t, err)
		assert.IsType(t, []map[string]interface{}{}, res)
		assert.Len(t, res, 3)
	}
}

type mockClientGetter struct {
	*fake.RESTClient
}

func (m mockClientGetter) UnversionedClient(_ *types.APIContext, _ types.StorageContext) (rest.Interface, error) {
	return m.RESTClient, nil
}

func (m mockClientGetter) APIExtClient(_ *types.APIContext, _ types.StorageContext) (clientset.Interface, error) {
	return nil, nil
}

func Test_shouldExpireAccessControl(t *testing.T) {
	req := &types.APIContext{}
	tests := []struct {
		name         string
		preFillCache func() // Optional setup function to put data in cache
		inputCtx     *types.APIContext
		want         bool
	}{
		{
			name:         "First request (Cache Miss)",
			preFillCache: nil, // Cache starts empty
			inputCtx:     req,
			want:         true, // Should return true and add to cache
		},
		{
			name: "Second request (Cache Hit)",
			preFillCache: func() {
				// Manually add req to the global cache before running the function
				lastExpiredRequests.Add(req, struct{}{}, 1*time.Minute)
			},
			inputCtx: req,
			want:     false,
		},
		{
			name: "Second request after expired (Cache Miss)",
			preFillCache: func() {
				// Manually add req to the global cache before running the function
				lastExpiredRequests.Add(req, struct{}{}, 100*time.Millisecond)
				time.Sleep(100 * time.Millisecond)
			},
			inputCtx: req,
			want:     true,
		},
		{
			name: "Different request (Cache Miss)",
			preFillCache: func() {
				lastExpiredRequests.Add(req, struct{}{}, 1*time.Minute)
			},
			inputCtx: &types.APIContext{},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				lastExpiredRequests.RemoveAll(func(any) bool {
					return true
				})
			})
			if tt.preFillCache != nil {
				tt.preFillCache()
			}

			if got := shouldExpireAccessControl(tt.inputCtx); got != tt.want {
				t.Errorf("shouldExpireAccessControl() = %v, want %v", got, tt.want)
			}
		})
	}
}
