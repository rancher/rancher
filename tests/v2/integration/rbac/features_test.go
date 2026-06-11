package integration

import (
	"errors"
	"net/http"

	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/pkg/clientbase"
)

func (p *RTBTestSuite) TestCannotCreateFeature() {
	client := p.newSubSession()

	// Create a standard user.
	user := p.createUser(client, "feature-user", "user")
	userClient, err := client.AsUser(user)
	p.Require().NoError(err)

	trueVal := true

	// Admin should not be able to create features (405 Method Not Allowed).
	_, err = client.Management.Feature.Create(&management.Feature{
		Name:  "testfeature",
		Value: &trueVal,
	})
	var apiErr *clientbase.APIError
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusMethodNotAllowed, apiErr.StatusCode)

	// Standard user should not be able to create features (405 Method Not Allowed).
	_, err = userClient.Management.Feature.Create(&management.Feature{
		Name:  "testfeature",
		Value: &trueVal,
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusMethodNotAllowed, apiErr.StatusCode)
}

func (p *RTBTestSuite) TestCanListFeatures() {
	client := p.newSubSession()

	// Create a standard user.
	user := p.createUser(client, "feature-user", "user")
	userClient, err := client.AsUser(user)
	p.Require().NoError(err)

	// Standard user should be able to list features.
	userFeatures, err := userClient.Management.Feature.List(nil)
	p.Require().NoError(err)
	p.Require().NotEmpty(userFeatures.Data)

	// Admin should be able to list features.
	adminFeatures, err := client.Management.Feature.List(nil)
	p.Require().NoError(err)
	p.Require().NotEmpty(adminFeatures.Data)
}
