package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/clientbase"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
)

type SettingsTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *SettingsTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *SettingsTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SettingsTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

// TestCreateReadOnly verifies that creating the readOnly "cacerts" setting is
// rejected with 405 Method Not Allowed.
func (s *SettingsTestSuite) TestCreateReadOnly() {
	_, err := s.client.Management.Setting.Create(&management.Setting{
		Name:  "cacerts",
		Value: "a",
	})
	s.Require().Error(err)
	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr))
	s.Equal(http.StatusMethodNotAllowed, apiErr.StatusCode)
	s.Contains(apiErr.Msg, "readOnly")
}

// TestUpdateReadOnly verifies that updating the readOnly "cacerts" setting is
// rejected with 405 Method Not Allowed.
func (s *SettingsTestSuite) TestUpdateReadOnly() {
	setting, err := s.client.Management.Setting.ByID("cacerts")
	s.Require().NoError(err)

	_, err = s.client.Management.Setting.Update(setting, &management.Setting{Value: "b"})
	s.Require().Error(err)
	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr))
	s.Equal(http.StatusMethodNotAllowed, apiErr.StatusCode)
	s.Contains(apiErr.Msg, "readOnly")
}

// TestGetReadOnly verifies that the readOnly "cacerts" setting can be retrieved.
func (s *SettingsTestSuite) TestGetReadOnly() {
	_, err := s.client.Management.Setting.ByID("cacerts")
	s.Require().NoError(err)
}

// TestDeleteReadOnly verifies that deleting the readOnly "cacerts" setting is
// rejected with 405 Method Not Allowed.
func (s *SettingsTestSuite) TestDeleteReadOnly() {
	setting, err := s.client.Management.Setting.ByID("cacerts")
	s.Require().NoError(err)

	err = s.client.Management.Setting.Delete(setting)
	s.Require().Error(err)
	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr))
	s.Equal(http.StatusMethodNotAllowed, apiErr.StatusCode)
	s.Contains(apiErr.Msg, "readOnly")
}

// TestCreate verifies that a new setting can be created with the expected value.
func (s *SettingsTestSuite) TestCreate() {
	subSession := s.session.NewSession()
	s.T().Cleanup(subSession.Cleanup)
	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	setting, err := client.Management.Setting.Create(&management.Setting{
		Name:  namegen.AppendRandomString("samplesetting-"),
		Value: "a",
	})
	s.Require().NoError(err)
	s.Equal("a", setting.Value)
}

// TestCreateExisting verifies that creating a setting whose name is already
// taken returns 409 Conflict with code AlreadyExists.
func (s *SettingsTestSuite) TestCreateExisting() {
	subSession := s.session.NewSession()
	s.T().Cleanup(subSession.Cleanup)
	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	name := namegen.AppendRandomString("samplesetting-")
	_, err = client.Management.Setting.Create(&management.Setting{
		Name:  name,
		Value: "a",
	})
	s.Require().NoError(err)

	_, err = client.Management.Setting.Create(&management.Setting{
		Name:  name,
		Value: "a",
	})
	s.Require().Error(err)
	var apiErr *clientbase.APIError
	s.Require().True(errors.As(err, &apiErr))
	s.Equal(http.StatusConflict, apiErr.StatusCode)
	s.Contains(apiErr.Msg, "AlreadyExists")
}

// TestUpdate verifies that an existing setting can be updated to a new value.
func (s *SettingsTestSuite) TestUpdate() {
	subSession := s.session.NewSession()
	s.T().Cleanup(subSession.Cleanup)
	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	setting, err := client.Management.Setting.Create(&management.Setting{
		Name:  namegen.AppendRandomString("samplesetting-"),
		Value: "a",
	})
	s.Require().NoError(err)

	updated, err := client.Management.Setting.Update(setting, &management.Setting{Value: "b"})
	s.Require().NoError(err)
	s.Equal("b", updated.Value)
}

// TestUpdateNonExisting verifies that attempting to update a setting that does
// not exist returns 404 Not Found.
func (s *SettingsTestSuite) TestUpdateNonExisting() {
	nonExistentID := namegen.AppendRandomString("nonexistent-")
	host := s.client.WranglerContext.RESTConfig.Host
	httpClient := s.httpClient()

	body, err := json.Marshal(map[string]any{"value": "a"})
	s.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("https://%s/v3/settings/%s", host, nonExistentID),
		bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	s.Require().NoError(err)
	io.ReadAll(resp.Body) //nolint:errcheck
	resp.Body.Close()
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

// TestUpdateLink verifies that the admin user sees the "update" action link on
// a setting, while a standard user does not.
func (s *SettingsTestSuite) TestUpdateLink() {
	subSession := s.session.NewSession()
	s.T().Cleanup(subSession.Cleanup)
	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	setting, err := client.Management.Setting.Create(&management.Setting{
		Name:  namegen.AppendRandomString("samplesetting-"),
		Value: "a",
	})
	s.Require().NoError(err)

	// Admin should see the update link.
	setting, err = client.Management.Setting.ByID(setting.ID)
	s.Require().NoError(err)
	_, hasUpdate := setting.Links["update"]
	s.True(hasUpdate, "admin should see update link on setting")

	// Create a standard user and verify they do not see the update link.
	enabled := true
	pw := password.GenerateUserPassword("testpass-")
	standardUser, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("user-"),
		Password: pw,
		Name:     "testuser",
		Enabled:  &enabled,
	}, "user")
	s.Require().NoError(err)
	standardUser.Password = pw

	userClient, err := client.AsUser(standardUser)
	s.Require().NoError(err)

	userSetting, err := userClient.Management.Setting.ByID(setting.ID)
	s.Require().NoError(err)
	_, hasUpdate = userSetting.Links["update"]
	s.False(hasUpdate, "standard user should not see update link on setting")
}

func TestSettings(t *testing.T) {
	suite.Run(t, new(SettingsTestSuite))
}
