package integration

import (
	"errors"
	"net/http"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/clientbase"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
)

type UserTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *UserTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)

	s.client = client
}

func (s *UserTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

// newSubSession creates a new sub-session client for test isolation.
func (s *UserTestSuite) newSubSession() *rancher.Client {
	subSession := s.session.NewSession()
	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)
	s.T().Cleanup(subSession.Cleanup)
	return client
}

func (s *UserTestSuite) TestUserCantDeleteSelf() {
	client := s.newSubSession()

	// Get the current admin user
	users, err := client.Management.User.List(nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(users.Data)

	// Find the admin user (the one we're authenticated as)
	var currentUser management.User
	found := false
	for _, user := range users.Data {
		if user.Username == "admin" {
			currentUser = user
			found = true
			break
		}
	}
	s.Require().True(found, "admin user not found")

	err = client.Management.User.Delete(&currentUser)
	s.Require().Error(err)

	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	s.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
}

func (s *UserTestSuite) TestUserCantDeactivateSelf() {
	client := s.newSubSession()

	users, err := client.Management.User.List(nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(users.Data)

	var currentUser management.User
	found := false
	for _, user := range users.Data {
		if user.Username == "admin" {
			found = true
			currentUser = user
			break
		}
	}
	s.Require().True(found, "admin user not found")

	enabled := false
	currentUser.Enabled = &enabled
	_, err = client.Management.User.Update(&currentUser, map[string]any{
		"enabled": false,
	})
	s.Require().Error(err)

	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	s.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
}

func (s *UserTestSuite) TestUserCantUseUsernameAsPassword() {
	client := s.newSubSession()

	enabled := true
	_, err := users.CreateUserWithRole(client, &management.User{
		Username: "administrator",
		Password: "administrator",
		Name:     "administrator",
		Enabled:  &enabled,
	}, "user")
	s.Require().Error(err)

	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	s.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
}

func (s *UserTestSuite) TestPasswordTooShort() {
	client := s.newSubSession()

	enabled := true
	username := namegen.AppendRandomString("testuser-")
	_, err := users.CreateUserWithRole(client, &management.User{
		Username: username,
		Password: "tooshort",
		Name:     username,
		Enabled:  &enabled,
	}, "user")
	s.Require().Error(err)

	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	s.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
}

func TestUserTestSuite(t *testing.T) {
	suite.Run(t, new(UserTestSuite))
}
