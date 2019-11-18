package clusterregistrationtokens

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/config"
	assertlib "github.com/stretchr/testify/assert"
	restclient "k8s.io/client-go/rest"
)

const (
	testAgentImage = "rancher/rancher-agent:test"
	testCertCAs    = `-----BEGIN CERTIFICATE-----
xxx
-----END CERTIFICATE-----`
)

func TestFormatter(t *testing.T) {
	testCases := []struct {
		caseName                    string
		presetAgentImage            string
		presetCertCAs               string
		presetServerURL             string
		presetSystemDefaultRegistry string
		requestURL                  string
		requestToken                string
		outputShouldEqual           map[string]interface{}
	}{
		{
			caseName:         "default",
			presetAgentImage: testAgentImage,
			presetCertCAs:    testCertCAs,
			presetServerURL:  "https://fake-server.rancher.io",
			requestURL:       "https://fake-test.rancher.io/v3/clusterregistrationtokens",
			requestToken:     "fake-token",
			outputShouldEqual: map[string]interface{}{
				"insecureCommand": fmt.Sprintf(insecureCommandFormat, "https://fake-server.rancher.io/v3/import/fake-token.yaml"),
				"command":         fmt.Sprintf(commandFormat, "https://fake-server.rancher.io/v3/import/fake-token.yaml"),
				"token":           "fake-token",
				"manifestUrl":     "https://fake-server.rancher.io/v3/import/fake-token.yaml",
				"nodeCommand": fmt.Sprintf(nodeCommandFormat,
					"rancher/rancher-agent:test",
					"https://fake-server.rancher.io",
					"fake-token",
					" --ca-checksum c1dedde8bea64bee49f62c595e6bf7a96eae0888cf73bb6c90b6be5031052600"),
				"windowsNodeCommand": fmt.Sprintf(windowsNodeCommandFormat,
					"",
					"rancher/rancher-agent:test",
					"https://fake-server.rancher.io",
					"fake-token",
					" --ca-checksum c1dedde8bea64bee49f62c595e6bf7a96eae0888cf73bb6c90b6be5031052600"),
			},
		},
		{
			caseName:                    "with private registry setting",
			presetAgentImage:            testAgentImage,
			presetCertCAs:               testCertCAs,
			presetServerURL:             "https://fake-server.rancher.io",
			presetSystemDefaultRegistry: "fake-registry.rancher.io:443",
			requestURL:                  "https://fake-test.rancher.io/v3/clusterregistrationtokens",
			requestToken:                "fake-token",
			outputShouldEqual: map[string]interface{}{
				"insecureCommand": fmt.Sprintf(insecureCommandFormat, "https://fake-server.rancher.io/v3/import/fake-token.yaml"),
				"command":         fmt.Sprintf(commandFormat, "https://fake-server.rancher.io/v3/import/fake-token.yaml"),
				"token":           "fake-token",
				"manifestUrl":     "https://fake-server.rancher.io/v3/import/fake-token.yaml",
				"nodeCommand": fmt.Sprintf(nodeCommandFormat,
					"fake-registry.rancher.io:443/rancher/rancher-agent:test",
					"https://fake-server.rancher.io",
					"fake-token",
					" --ca-checksum c1dedde8bea64bee49f62c595e6bf7a96eae0888cf73bb6c90b6be5031052600"),
				"windowsNodeCommand": fmt.Sprintf(windowsNodeCommandFormat,
					"-e AGENT_IMAGE=fake-registry.rancher.io:443/rancher/rancher-agent:test ",
					"fake-registry.rancher.io:443/rancher/rancher-agent:test",
					"https://fake-server.rancher.io",
					"fake-token",
					" --ca-checksum c1dedde8bea64bee49f62c595e6bf7a96eae0888cf73bb6c90b6be5031052600"),
			},
		},
		{
			caseName:         "without server URL setting",
			presetAgentImage: testAgentImage,
			presetCertCAs:    testCertCAs,
			presetServerURL:  "",
			requestURL:       "https://fake-test.rancher.io/v3/clusterregistrationtokens",
			requestToken:     "fake-token",
			outputShouldEqual: map[string]interface{}{
				"insecureCommand": fmt.Sprintf(insecureCommandFormat, "https://fake-test.rancher.io/v3/import/fake-token.yaml"),
				"command":         fmt.Sprintf(commandFormat, "https://fake-test.rancher.io/v3/import/fake-token.yaml"),
				"token":           "fake-token",
				"manifestUrl":     "https://fake-test.rancher.io/v3/import/fake-token.yaml",
				"nodeCommand": fmt.Sprintf(nodeCommandFormat,
					"rancher/rancher-agent:test",
					"https://fake-test.rancher.io",
					"fake-token",
					" --ca-checksum c1dedde8bea64bee49f62c595e6bf7a96eae0888cf73bb6c90b6be5031052600"),
				"windowsNodeCommand": fmt.Sprintf(windowsNodeCommandFormat,
					"",
					"rancher/rancher-agent:test",
					"https://fake-test.rancher.io",
					"fake-token",
					" --ca-checksum c1dedde8bea64bee49f62c595e6bf7a96eae0888cf73bb6c90b6be5031052600"),
			},
		},
	}

	assert := assertlib.New(t)
	for _, cs := range testCases {
		// configure settings
		err := settings.AgentImage.Set(cs.presetAgentImage)
		assert.Nilf(err, "%s could not set fake AgentImage", cs.caseName)
		err = settings.CACerts.Set(cs.presetCertCAs)
		assert.Nilf(err, "%s could not set fake CACerts", cs.caseName)
		err = settings.ServerURL.Set(cs.presetServerURL)
		assert.Nilf(err, "%s could not set fake ServerURL", cs.caseName)
		err = settings.SystemDefaultRegistry.Set(cs.presetSystemDefaultRegistry)
		assert.Nilf(err, "%s could not set fake ServerURL", cs.caseName)

		// prepare
		fakeAPIContext, err := newFakeAPIContext(cs.requestURL)
		assert.Nilf(err, "%s could not new fake APIContext", cs.caseName)
		fakeRawResource, err := newFakeRawResource(cs.requestToken)
		assert.Nilf(err, "%s could not new fake RawResource", cs.caseName)

		// verify
		scaledContext, err := config.NewScaledContext(restclient.Config{})
		assert.Nil(err)
		assert.NotNil(scaledContext)
		tokenFormatter := NewFormatter(scaledContext)
		tokenFormatter.Formatter(fakeAPIContext, fakeRawResource)
		assert.Equal(cs.outputShouldEqual, fakeRawResource.Values, cs.caseName)
	}

}

func newFakeAPIContext(url string) (*types.APIContext, error) {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	version := types.APIVersion{}
	urlB, err := urlbuilder.New(req, version, nil)
	if err != nil {
		return nil, err
	}

	apiContext := &types.APIContext{
		URLBuilder: urlB,
	}
	return apiContext, nil
}

func newFakeRawResource(token string) (*types.RawResource, error) {
	values := map[string]interface{}{
		"token": token,
	}

	rawResource := &types.RawResource{
		Values: values,
	}
	return rawResource, nil
}
