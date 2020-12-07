package notifiers

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/stretchr/testify/assert"
)

func TestIsHTTPClientConfigSet(t *testing.T) {
	type testCase struct {
		httpConfig *v32.HTTPClientConfig
		want       bool
	}

	testCases := []testCase{
		testCase{
			httpConfig: &v32.HTTPClientConfig{
				ProxyURL: "test",
			},
			want: true,
		},
		testCase{
			httpConfig: &v32.HTTPClientConfig{
				ProxyURL: "",
			},
			want: false,
		},
		testCase{
			httpConfig: nil,
			want:       false,
		},
	}

	assert := assert.New(t)
	for _, tcase := range testCases {
		assert.Equal(IsHTTPClientConfigSet(tcase.httpConfig), tcase.want)
	}

}
