//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package userretention

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UserRetentionSettingsTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	originalTTL   string
	settingsReset bool
}

func (ur *UserRetentionSettingsTestSuite) SetupSuite() {
	ur.session = session.NewSession()
	client, err := rancher.NewClient("", ur.session)
	require.NoError(ur.T(), err)
	ur.client = client

	settings, err := ur.client.Management.Setting.ByID(authUserSessionTTLMinutes)
	require.NoError(ur.T(), err)
	ur.originalTTL = settings.Value

	if ur.originalTTL != defaultAuthSessionTTL {
		err = updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, defaultAuthSessionTTL)
		require.NoError(ur.T(), err)
		ur.settingsReset = true

		newTTL, err := ur.getAuthSessionTTL()
		require.NoError(ur.T(), err)
		require.Equal(ur.T(), defaultAuthSessionTTL, newTTL, "Failed to verify default TTL was set")
	}

	err = updateUserRetentionSettings(ur.client, disableInactiveUserAfter, "")
	require.NoError(ur.T(), err, "Failed to reset disable-inactive-user-after setting")

	err = updateUserRetentionSettings(ur.client, deleteInactiveUserAfter, "")
	require.NoError(ur.T(), err, "Failed to reset delete-inactive-user-after setting")

	err = updateUserRetentionSettings(ur.client, userRetentionCron, "0 * * * *")
	require.NoError(ur.T(), err, "Failed to reset user-retention-cron setting")

	ttl, err := ur.getAuthSessionTTL()
	require.NoError(ur.T(), err)
	require.Equal(ur.T(), defaultAuthSessionTTL, ttl, "Auth session TTL not set to default value")

	logrus.Infof("Test suite initialized with default TTL: %s", defaultAuthSessionTTL)
}

func (ur *UserRetentionSettingsTestSuite) TearDownSuite() {
	err := updateUserRetentionSettings(ur.client, disableInactiveUserAfter, "")
	require.NoError(ur.T(), err)

	err = updateUserRetentionSettings(ur.client, deleteInactiveUserAfter, "")
	require.NoError(ur.T(), err)

	err = updateUserRetentionSettings(ur.client, userRetentionCron, "0 * * * *")
	require.NoError(ur.T(), err)

	if ur.settingsReset {
		err := updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, ur.originalTTL)
		require.NoError(ur.T(), err)

		finalTTL, err := ur.getAuthSessionTTL()
		require.NoError(ur.T(), err)
		require.Equal(ur.T(), ur.originalTTL, finalTTL, "Failed to restore original TTL")
	}

	ur.session.Cleanup()
}

func (ur *UserRetentionSettingsTestSuite) getAuthSessionTTL() (string, error) {
	settings, err := ur.client.Management.Setting.ByID(authUserSessionTTLMinutes)
	if err != nil {
		return "", err
	}
	return settings.Value, nil
}

func (ur *UserRetentionSettingsTestSuite) ensureDefaultTTL() error {
	return updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, defaultAuthSessionTTL)
}

func (ur *UserRetentionSettingsTestSuite) testPositiveInputValues(settingName string, tests []struct {
	name        string
	value       string
	description string
}) {
	logrus.Infof("Updating %s settings with positive values:", settingName)
	for _, inputValue := range tests {
		ur.T().Run(inputValue.name, func(*testing.T) {
			err := updateUserRetentionSettings(ur.client, settingName, inputValue.value)
			assert.NoError(ur.T(), err, "Unexpected error for input '%s'", inputValue.value)

			if err == nil {
				settings, err1 := ur.client.Management.Setting.ByID(settingName)
				if assert.NoError(ur.T(), err1, "Failed to retrieve settings") {
					assert.Equal(ur.T(), settingName, settings.Name)
					assert.Equal(ur.T(), inputValue.value, settings.Value)
				}
			}
			logrus.Infof("%s setting is updated to %s ; %s", settingName, inputValue.value, inputValue.description)
		})
	}
}

func (ur *UserRetentionSettingsTestSuite) testNegativeInputValues(settingName string, tests []struct {
	name        string
	value       string
	description string
}) {
	logrus.Infof("Updating %s settings with negative values:", settingName)
	for _, inputValue := range tests {
		ur.T().Run(inputValue.name, func(*testing.T) {
			err := updateUserRetentionSettings(ur.client, settingName, inputValue.value)
			assert.Error(ur.T(), err, "Expected an error for input '%s', but got nil", inputValue.value)

			if err != nil {
				ur.validateError(err, inputValue.description)
				ur.validateSettingsNotUpdated(settingName, inputValue.value)
			}
			logrus.Infof("Failed to update %s settings to %s; %s", settingName, inputValue.value, inputValue.description)
		})
	}
}

func (ur *UserRetentionSettingsTestSuite) validateError(err error, expectedDescription string) {
	errMsg := err.Error()
	if !assert.Contains(ur.T(), errMsg, expectedDescription, "Error message mismatch") {
		logrus.Infof("Expected error to contain: %s", expectedDescription)
		logrus.Infof("Actual error message: %s", errMsg)
	}
}

func (ur *UserRetentionSettingsTestSuite) validateSettingsNotUpdated(settingName, inputValue string) {
	settings, err := ur.client.Management.Setting.ByID(settingName)
	if assert.NoError(ur.T(), err, "Failed to retrieve settings") {
		assert.Equal(ur.T(), settingName, settings.Name)
		assert.NotEqual(ur.T(), inputValue, settings.Value)
	}
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForAuthUserSessionTTLWithPositiveInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"TTLUpdatedDefault", defaultAuthSessionTTL, "Default 16 hour session"},
		{"TTLUpdatedOneDay", "1440", "24 hour session"},
		{"TTLUpdatedOneWeek", "10080", "One week session"},
	}
	ur.testPositiveInputValues(authUserSessionTTLMinutes, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForAuthUserSessionTTLWithNegativeInputValues() {
	require.NoError(ur.T(), ur.ensureDefaultTTL())
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"TTLErrorNegative", "-10", "Invalid value: \"-10\": negative value"},
		{"TTLErrorNonNumeric", "abc", "strconv.ParseInt: parsing \"abc\": invalid syntax"},
		{"TTLErrorDecimal", "10.5", "strconv.ParseInt: parsing \"10.5\": invalid syntax"},
		{"TTLErrorLarge", "999999999999999", "Invalid value: \"999999999999999\": negative value"},
		{"TTLErrorSpecialChars", "10@20", "strconv.ParseInt: parsing \"10@20\": invalid syntax"},
	}
	ur.testNegativeInputValues(authUserSessionTTLMinutes, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDisableInactiveUserAfterWithNegativeInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DisableAfterUpdateErrorMissingUnit", "10", "time: missing unit in duration"},
		{"DisableAfterUpdateErrorInvalidUnitS", "10S", "time: unknown unit"},
		{"DisableAfterUpdateErrorInvalidUnitM", "10M", "time: unknown unit"},
		{"DisableAfterUpdateErrorInvalidUnitH", "10H", "time: unknown unit"},
		{"DisableAfterUpdateErrorNegativeDuration", "-20m", "negative value"},
		{"DisableAfterUpdateErrorInvalidDuration", "tens", "invalid duration"},
		{"DisableAfterUpdateErrorBelowMinimum", "15h", "can't be less than auth-user-session-ttl-minutes"},
	}
	ur.testNegativeInputValues(disableInactiveUserAfter, tests)
}
func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDisableInactiveUserAfterWithPositiveInputValues() {
	minDuration := "25h"

	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DisableAfterUpdatedNoAction", "", "No action - users will not be deactivated"},
		{"DisableAfterUpdatedAboveMinimum", minDuration, "Users will be deactivated after minimum required duration"},
		{"DisableAfterUpdatedTwoDays", "48h", "Users will be deactivated after 48h"},
		{"DisableAfterUpdatedTenThousandHours", "10000h", "Users will be deactivated after 10000h"},
	}
	ur.testPositiveInputValues(disableInactiveUserAfter, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDeleteInactiveUserAfterWithNegativeInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DeleteErrorMissingUnit", "10", "time: missing unit in duration"},
		{"DeleteErrorTooShort", "335h", "Forbidden: must be at least 336h0m0s"},
		{"DeleteErrorBelowTTL", "15h", "Forbidden: must be at least 336h0m0s"},
		{"DeleteErrorInvalidUnit", "1d", "time: unknown unit"},
		{"DeleteErrorNegative", "-20h", "negative value"},
	}
	ur.testNegativeInputValues(deleteInactiveUserAfter, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForUserRetentionCronWithPositiveInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"CronRunsEvery1Hour", "0 * * * *", "every 1 hour"},
		{"CronRunsEvery1Day", "0 0 * * *", "every 1 day"},
		{"CronRunsEvery5Minutes", "*/5 * * * *", "every 5 mins"},
		{"CronRunsEveryMinute", "* * * * *", "every min"},
		{"CronRunsEvery2PMTo205PM", "0-5 14 * * *", "every minute 2:00 PM to 2:05 PM"},
		{"CronRunsFirstSecondDayMidnight", "0 0 1,2 * *", "midnight of 1st, 2nd day"},
	}
	ur.testPositiveInputValues(userRetentionCron, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForUserRetentionCronWithNegativeInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"CronUpdateErrorTooManyFields", "* * * * * *", "Invalid value: \"* * * * * *\": Expected exactly 5 fields"},
		{"CronUpdateErrorNegativeNumber", "*/-1 * * * *", "Invalid value: \"*/-1 * * * *\": Negative number (-1) not allowed: -1"},
		{"CronUpdateErrorOutOfRange", "60/1 * * * *", "Invalid value: \"60/1 * * * *\": Beginning of range (60) beyond end of range (59): 60/1"},
		{"CronUpdateErrorLessFields", "10min", "Invalid value: \"10min\": Expected exactly 5 fields"},
		{"CronUpdateErrorInvalidSyntax", "(*/1) * * * *", "Failed to parse int from"},
	}
	ur.testNegativeInputValues(userRetentionCron, tests)
}

func TestUserRetentionSettingsSuite(t *testing.T) {
	suite.Run(t, new(UserRetentionSettingsTestSuite))
}
