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
	tests := []struct {
		name        string
		value       string
		description string
	}{
		{"DisableAfterUpdatedNoAction", "", "No action - users will not be deactivated"},
		{"DisableAfterUpdatedZeroSeconds", "0s", "Users will be deactivated after 0s"},
		{"DisableAfterUpdatedZeroMinutes", "0m", "Users will be deactivated after 0m"},
		{"DisableAfterUpdatedZeroHours", "0h", "Users will be deactivated after 0h"},
		{"DisableAfterUpdatedTenSeconds", "10s", "Users will be deactivated after 10s"},
		{"DisableAfterUpdatedTenMinutes", "10m", "Users will be deactivated after 10m"},
		{"DisableAfterUpdatedTwentyHours", "20h", "Users will be deactivated after 20h"},
		{"DisableAfterUpdatedTenThousandSeconds", "10000s", "Users will be deactivated after 10000s"},
		{"DisableAfterUpdatedTenThousandMinutes", "10000m", "Users will be deactivated after 10000m"},
		{"DisableAfterUpdatedTenThousandHours", "10000h", "Users will be deactivated after 10000h"},
	}
	ur.testPositiveInputValues(disableInactiveUserAfter, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDisableInactiveUserAfterWithNegativeInputValues() {
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
		{"DisableAfterUpdateErrorNegativeDuration", "-20m", "Invalid value: \"-20m\": negative duration"},
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
		{"DeleteErrorTooShortSeconds", "10s", "Invalid value: \"10s\": must be at least 336h0m0s"},
		{"DeleteErrorTooShortMinutes", "10m", "Invalid value: \"10m\": must be at least 336h0m0s"},
		{"DeleteErrorTooShortHours", "10h", "Invalid value: \"10h\": must be at least 336h0m0s"},
		{"DeleteErrorInvalidUnitS", "10S", "Invalid value: \"10S\": time: unknown unit \"S\" in duration \"10S\""},
		{"DeleteErrorInvalidUnitM", "10M", "Invalid value: \"10M\": time: unknown unit \"M\" in duration \"10M\""},
		{"DeleteErrorInvalidUnitH", "10H", "Invalid value: \"10H\": time: unknown unit \"H\" in duration \"10H\""},
		{"DeleteErrorInvalidUnitSec", "10sec", "Invalid value: \"10sec\": time: unknown unit \"sec\" in duration \"10sec\""},
		{"DeleteErrorInvalidUnitMin", "10min", "Invalid value: \"10min\": time: unknown unit \"min\" in duration \"10min\""},
		{"DeleteErrorInvalidUnitHour", "20hour", "Invalid value: \"20hour\": time: unknown unit \"hour\" in duration \"20hour\""},
		{"DeleteErrorInvalidUnitDay", "1d", "Invalid value: \"1d\": time: unknown unit \"d\" in duration \"1d\""},
		{"DeleteErrorNegativeDuration", "-20m", "Invalid value: \"-20m\": negative duration"},
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

func TestUserRetentionSettingsSuite(t *testing.T) {
	suite.Run(t, new(UserRetentionSettingsTestSuite))
}
