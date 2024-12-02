//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package userretention

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type UserRetentionSettingsTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (ur *UserRetentionSettingsTestSuite) SetupSuite() {
	ur.session = session.NewSession()
	client, err := rancher.NewClient("", ur.session)
	require.NoError(ur.T(), err)
	ur.client = client

	err = updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, "0")
	require.NoError(ur.T(), err)
}

func (ur *UserRetentionSettingsTestSuite) TearDownSuite() {
	ur.session.Cleanup()
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
	var statusErr *apierrors.StatusError
	var found bool

	switch e := err.(type) {
	case interface{ Unwrap() error }:
		if innerErr := e.Unwrap(); innerErr != nil {
			if innerWrapper, ok := innerErr.(interface{ Unwrap() error }); ok {
				if deepestErr := innerWrapper.Unwrap(); deepestErr != nil {
					statusErr, found = deepestErr.(*apierrors.StatusError)
				}
			}
		}
	}

	if found && statusErr != nil {
		assert.Equal(ur.T(), int32(400), statusErr.ErrStatus.Code, "Status code should be 400")
		assert.Equal(ur.T(), metav1.StatusReasonBadRequest, statusErr.ErrStatus.Reason, "Reason should be BadRequest")
		assert.Contains(ur.T(), statusErr.ErrStatus.Message, expectedDescription, "Error should contain the expected description")
	} else {
		errMsg := err.Error()
		assert.Contains(ur.T(), errMsg, "denied the request", "Error should mention denied request")
		assert.Contains(ur.T(), errMsg, expectedDescription, "Error should contain the expected description")
	}
}

func (ur *UserRetentionSettingsTestSuite) validateSettingsNotUpdated(settingName, inputValue string) {
	settings, err := ur.client.Management.Setting.ByID(settingName)
	if assert.NoError(ur.T(), err, "Failed to retrieve settings") {
		assert.Equal(ur.T(), settingName, settings.Name)
		assert.NotEqual(ur.T(), inputValue, settings.Value)
	}
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDisableInactiveUserAfterWithPositiveInputValues() {

	err := updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, "1")
	require.NoError(ur.T(), err)

	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DisableAfterUpdatedNoAction", "", "No action - users will not be deactivated"},
		{"DisableAfterUpdatedZeroSeconds", "0s", "Users will be deactivated after 0s"},
		{"DisableAfterUpdatedZeroMinutes", "0m", "Users will be deactivated after 0m"},
		{"DisableAfterUpdatedZeroHours", "0h", "Users will be deactivated after 0h"},
		{"DisableAfterUpdatedTenSeconds", "60s", "Users will be deactivated after 60s"},
		{"DisableAfterUpdatedTenMinutes", "10m", "Users will be deactivated after 10m"},
		{"DisableAfterUpdatedTwentyHours", "20h", "Users will be deactivated after 20h"},
		{"DisableAfterUpdatedTenThousandSeconds", "10000s", "Users will be deactivated after 10000s"},
		{"DisableAfterUpdatedTenThousandMinutes", "10000m", "Users will be deactivated after 10000m"},
		{"DisableAfterUpdatedTenThousandHours", "10000h", "Users will be deactivated after 10000h"},
	}
	ur.testPositiveInputValues(disableInactiveUserAfter, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDisableInactiveUserAfterWithNegativeInputValues() {
	err := updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, "1")
	require.NoError(ur.T(), err)

	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DisableAfterUpdateErrorMissingUnit", "10", "Invalid value: \"10\": time: missing unit in duration \"10\""},
		{"DisableAfterUpdateErrorInvalidUnitS", "10S", "Invalid value: \"10S\": time: unknown unit \"S\" in duration \"10S\""},
		{"DisableAfterUpdateErrorInvalidUnitM", "10M", "Invalid value: \"10M\": time: unknown unit \"M\" in duration \"10M\""},
		{"DisableAfterUpdateErrorInvalidUnitH", "10H", "Invalid value: \"10H\": time: unknown unit \"H\" in duration \"10H\""},
		{"DisableAfterUpdateErrorInvalidUnitSec", "10sec", "Invalid value: \"10sec\": time: unknown unit \"sec\" in duration \"10sec\""},
		{"DisableAfterUpdateErrorInvalidUnitMin", "10min", "Invalid value: \"10min\": time: unknown unit \"min\" in duration \"10min\""},
		{"DisableAfterUpdateErrorInvalidUnitHour", "20hour", "Invalid value: \"20hour\": time: unknown unit \"hour\" in duration \"20hour\""},
		{"DisableAfterUpdateErrorInvalidUnitDay", "1d", "Invalid value: \"1d\": time: unknown unit \"d\" in duration \"1d\""},
		{"DisableAfterUpdateErrorNegativeDuration", "-20m", "Invalid value: \"-20m\": negative value"},
		{"DisableAfterUpdateErrorInvalidDuration", "tens", "Invalid value: \"tens\": time: invalid duration \"tens\""},
	}
	ur.testNegativeInputValues(disableInactiveUserAfter, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDeleteInactiveUserAfterWithPositiveInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DeleteAfterNoAction", "", "No action - users will not be deleted"},
		{"DeleteAfterHundredMillionSeconds", "100000000s", "Users will delete after 100000000s"},
		{"DeleteAfterTwoHundredThousandMinutes", "200000m", "Users will delete after 200000m"},
		{"DeleteAfterTenThousandHours", "10000h", "Users will delete after 10000h"},
	}
	ur.testPositiveInputValues(deleteInactiveUserAfter, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDeleteInactiveUserAfterWithNegativeInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DeleteErrorMissingUnit", "10", "Invalid value: \"10\": time: missing unit in duration \"10\""},
		{"DeleteErrorTooShortSeconds", "10s", "Forbidden: must be at least 336h0m0s"},
		{"DeleteErrorTooShortMinutes", "10m", "Forbidden: must be at least 336h0m0s"},
		{"DeleteErrorTooShortHours", "10h", "Forbidden: must be at least 336h0m0s"},
		{"DeleteErrorInvalidUnitS", "10S", "time: unknown unit"},
		{"DeleteErrorInvalidUnitM", "10M", "time: unknown unit"},
		{"DeleteErrorInvalidUnitH", "10H", "time: unknown unit"},
		{"DeleteErrorInvalidUnitSec", "10sec", "time: unknown unit"},
		{"DeleteErrorInvalidUnitMin", "10min", "time: unknown unit"},
		{"DeleteErrorInvalidUnitHour", "20hour", "time: unknown unit"},
		{"DeleteErrorInvalidUnitDay", "1d", "time: unknown unit"},
		{"DeleteErrorNegativeDuration", "-20m", "negative value"},
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
		{"CronRunsEvery1Minute", "*/1 * * * *", "every min"},
		{"CronRunsEveryMinute", "* * * * *", "every min"},
		{"CronRunsEvery30Seconds", "30/1 * * * *", "every 30 sec"},
		{"CronRunsEvery2PMTo205PM", "0-5 14 * * *", "every minute starting at 2:00 PM and ending at 2:05 PM, every day"},
		{"CronRunsFirstSecondDayMidnight", "0 0 1,2 * *", "at midnight of 1st, 2nd day of each month"},
		{"CronRunsFirstSecondDayWednesdayMidnight", "0 0 1,2 * 3", "at midnight of 1st, 2nd day of each month, and each Wednesday"},
	}
	ur.testPositiveInputValues(userRetentionCron, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForUserRetentionCronWithNegativeInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"CronUpdateErrorTooManyFields", "* * * * * *", "Invalid value: \"* * * * * *\": Expected exactly 5 fields, found 6: * * * * * *"},
		{"CronUpdateErrorNegativeNumber", "*/-1 * * * *", "Invalid value: \"*/-1 * * * *\": Negative number (-1) not allowed: -1"},
		{"CronUpdateErrorOutOfRange", "60/1 * * * *", "Invalid value: \"60/1 * * * *\": Beginning of range (60) beyond end of range (59): 60/1"},
		{"CronUpdateErrorNegativeStart", "-30/1 * * * *", "Invalid value: \"-30/1 * * * *\": Failed to parse int from : strconv.Atoi: parsing \"\": invalid syntax"},
		{"CronUpdateErrorInvalidSyntax", "(*/1) * * * *", "Invalid value: \"(*/1) * * * *\": Failed to parse int from (*: strconv.Atoi: parsing \"(*\": invalid syntax"},
		{"CronUpdateErrorTooManyFields", "* * * * * */2", "Invalid value: \"* * * * * */2\": Expected exactly 5 fields, found 6: * * * * * */2"},
		{"CronUpdateErrorLessFields1", "10min", "Invalid value: \"10min\": Expected exactly 5 fields, found 1: 10min"},
		{"CronUpdateErrorLessFields2", "1d", "Invalid value: \"1d\": Expected exactly 5 fields, found 1: 1d"},
		{"CronUpdateErrorInvalidNegative", "-20m", "Invalid value: \"-20m\": Expected exactly 5 fields, found 1: -20m"},
	}
	ur.testNegativeInputValues(userRetentionCron, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForAuthUserSessionTTLWithPositiveInputValues() {

	err := setupUserRetentionSettings(ur.client, "1600h", "1600h", "*/1 * * * *", "false")
	require.NoError(ur.T(), err)

	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"TTLUpdatedMin", "1", "Minimum 1 minute session"},
		{"TTLUpdatedHour", "60", "One hour session"},
		{"TTLUpdatedDay", "1440", "24 hour session"},
		{"TTLUpdatedWeek", "10080", "One week session"},
	}
	ur.testPositiveInputValues(authUserSessionTTLMinutes, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForAuthUserSessionTTLWithNegativeInputValues() {
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"TTLErrorNegative", "-10", "negative value"},
		{"TTLErrorNonNumeric", "abc", "strconv.ParseInt: parsing \"abc\": invalid syntax"},
		{"TTLErrorDecimal", "10.5", "strconv.ParseInt: parsing \"10.5\": invalid syntax"},
		{"TTLErrorSpecialChars", "10@20", "strconv.ParseInt: parsing \"10@20\": invalid syntax"},
	}
	ur.testNegativeInputValues(authUserSessionTTLMinutes, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestDisableInactiveUserLessThanTTL() {
	err := setupUserRetentionSettings(ur.client, "1600m", "1600h", "*/1 * * * *", "false")
	require.NoError(ur.T(), err)

	err = updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, "1600")
	require.NoError(ur.T(), err)

	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DisableLessThanTTL30m", "30m", "Forbidden: can't be less than auth-user-session-ttl-minutes"},
		{"DisableLessThanTTL599m", "599m", "Forbidden: can't be less than auth-user-session-ttl-minutes"},
		{"DisableLessThanTTL1h", "1h", "Forbidden: can't be less than auth-user-session-ttl-minutes"},
	}
	ur.testNegativeInputValues(disableInactiveUserAfter, tests)

	err = updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, testTTLValue)
	require.NoError(ur.T(), err)
}

func (ur *UserRetentionSettingsTestSuite) TestDeleteInactiveUserLessThanTTL() {
	err := setupUserRetentionSettings(ur.client, "21600m", "1600h", "*/1 * * * *", "false")
	require.NoError(ur.T(), err)

	err = updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, "21600")
	require.NoError(ur.T(), err)

	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DeleteLessThanTTL4h", "338h", "Forbidden: can't be less than auth-user-session-ttl-minutes"},
		{"DeleteLessThanTTL359m", "359h", "Forbidden: can't be less than auth-user-session-ttl-minutes"},
		{"DeleteLessThanTTL5h", "5h", "Forbidden: must be at least 336h0m0s"},
	}
	ur.testNegativeInputValues(deleteInactiveUserAfter, tests)

	err = updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, testTTLValue)
	require.NoError(ur.T(), err)
}

func (ur *UserRetentionSettingsTestSuite) TestInactiveUserSettingsGreaterThanTTL() {
	err := updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, "60")
	require.NoError(ur.T(), err)

	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"ValidDisableAfter2h", "2h", "Valid value greater than TTL"},
		{"ValidDisableAfter61m", "61m", "Valid value greater than TTL"},
		{"ValidDisableAfter3600s", "3600s", "Valid value greater than TTL"},
	}
	ur.testPositiveInputValues(disableInactiveUserAfter, tests)

	deleteTests := []struct {
		name        string
		value       string
		description string
	}{
		{"ValidDeleteAfter337h", "337h", "Valid value greater than minimum and TTL"},
		{"ValidDeleteAfter20160m", "20160m", "Valid value greater than minimum and TTL"},
	}
	ur.testPositiveInputValues(deleteInactiveUserAfter, deleteTests)

	err = updateUserRetentionSettings(ur.client, authUserSessionTTLMinutes, testTTLValue)
	require.NoError(ur.T(), err)
}

func TestUserRetentionSettingsSuite(t *testing.T) {
	suite.Run(t, new(UserRetentionSettingsTestSuite))
}
