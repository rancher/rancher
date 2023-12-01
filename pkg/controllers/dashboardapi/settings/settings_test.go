package settings

import (
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

type testCase struct {
	description     string
	envVar          *string
	newDefVal       string
	newSetting      settings.Setting
	existingSetting *v3.Setting
}

func TestSetAll(t *testing.T) {
	settingsETCD := make(map[string]*v3.Setting)

	settingClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	testSettingsProvider := settingsProvider{
		settings: settingClient,
	}

	testCases := populateTestCases()
	settingClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options metav1.GetOptions) (*v3.Setting, error) {
		val := settingsETCD[name]
		if val == nil {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		return val, nil
	}).AnyTimes()

	settingClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(setting *v3.Setting) (*v3.Setting, error) {
		settingsETCD[setting.Name] = setting
		return setting, nil
	}).AnyTimes()

	settingClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *v3.Setting) (*v3.Setting, error) {
		settingsETCD[s.Name] = s
		return s, nil
	}).AnyTimes()

	settingClient.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*v3.SettingList, error) {
		var items []v3.Setting
		for _, setting := range settingsETCD {
			items = append(items, *setting)
		}

		return &v3.SettingList{Items: items}, nil
	}).Times(1)

	settingMap := make(map[string]settings.Setting)
	for _, test := range testCases {
		settingMap[test.newSetting.Name] = test.newSetting
		if test.envVar != nil {
			envKey := settings.GetEnvKey(test.newSetting.Name)
			os.Setenv(envKey, *test.envVar)
			defer os.Unsetenv(envKey)
		}

		settingsETCD[test.newSetting.Name] = test.existingSetting
	}

	settingsETCD["unknown"] = &v3.Setting{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unknown",
		},
		Value:   "unknown",
		Default: "unknown",
	}

	err := testSettingsProvider.SetAll(settingMap)
	assert.Nil(t, err, "set all should not return an error")

	for _, test := range testCases {
		finalSetting, err := testSettingsProvider.settings.Get(test.newSetting.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		fallbackValue := testSettingsProvider.fallback[test.newSetting.Name]
		failMsg := fmt.Sprintf("test case failed [%s]: %s", test.newSetting.Name, test.description)
		fallbackFailMsg := fmt.Sprintf("test case failed [%s]: fallback value not properly set", test.newSetting.Name)

		// updating setting in kubernetes should have the default from newSetting
		assert.Equal(t, finalSetting.Default, test.newSetting.Default, failMsg)

		// if the value is configured by an environment variable, then the source should be "env", otherwise it should be empty
		assert.True(t, finalSetting.Source == "env" == (test.envVar != nil), failMsg)

		var expectedFallbackVal string
		if test.envVar != nil {
			// environment variable takes precedence of everything. Setting's value should match as long as it was set.
			assert.Equal(t, *test.envVar, finalSetting.Value, failMsg)
			expectedFallbackVal = *test.envVar
		} else if test.existingSetting != nil {
			expectedFallbackVal = test.existingSetting.Value
			assert.Equal(t, test.existingSetting.Value, finalSetting.Value, failMsg)
		} else {
			assert.Equal(t, "", finalSetting.Value, failMsg)
		}

		if expectedFallbackVal == "" {
			// fallback value should be equal to default if value is empty. This is how clients of the settings provider
			// evaluate the effective value of the setting.
			expectedFallbackVal = test.newSetting.Default
		}

		assert.Equal(t, expectedFallbackVal, fallbackValue, fallbackFailMsg)
	}

	assert.NotNil(t, settingsETCD["unknown"].Labels)
	assert.Equal(t, settingsETCD["unknown"].Labels["cattle.io/unknown"], "true")

	cannotCreateClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	testSettingsProvider = settingsProvider{
		settings: cannotCreateClient,
	}
	cannotCreateClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options metav1.GetOptions) (*v3.Setting, error) {
		val := settingsETCD[name]
		if val == nil {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		return val, nil
	}).AnyTimes()

	// Test when setting client's Create method fails
	cannotCreateClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(setting *v3.Setting) (*v3.Setting, error) {
		return nil, apierrors.NewServiceUnavailable("some error")
	}).AnyTimes()

	cannotCreateClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *v3.Setting) (*v3.Setting, error) {
		settingsETCD[s.Name] = s
		return s, nil
	}).AnyTimes()

	settingsETCD = make(map[string]*v3.Setting)
	err = testSettingsProvider.SetAll(settingMap)
	assert.NotNilf(t, err, "SetAll should return an error if setting client's Create returns an error that is IsAlreadyExists.")

	cannotUpdateClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	testSettingsProvider = settingsProvider{
		settings: cannotCreateClient,
	}

	cannotUpdateClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options metav1.GetOptions) (*v3.Setting, error) {
		val := settingsETCD[name]
		if val == nil {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		return val, nil
	}).AnyTimes()

	cannotUpdateClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(setting *v3.Setting) (*v3.Setting, error) {
		settingsETCD[setting.Name] = setting
		return setting, nil
	}).AnyTimes()

	// Test when setting client's Update method fails
	cannotUpdateClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *v3.Setting) (*v3.Setting, error) {
		return nil, apierrors.NewServiceUnavailable("some error")
	}).AnyTimes()

	settingsETCD = make(map[string]*v3.Setting)

	err = testSettingsProvider.SetAll(settingMap)
	assert.NotNilf(t, err, "SetAll should return an error if setting client's Update returns an error.")

	alreadyExistsCreateClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	testSettingsProvider = settingsProvider{
		settings: alreadyExistsCreateClient,
	}

	alreadyExistsCreateClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options metav1.GetOptions) (*v3.Setting, error) {
		val := settingsETCD[name]
		if val == nil {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		return val, nil
	}).AnyTimes()

	// Test when setting client's Update method fails with AlreadyExists error
	alreadyExistsCreateClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(setting *v3.Setting) (*v3.Setting, error) {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "some error")
	}).AnyTimes()

	alreadyExistsCreateClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *v3.Setting) (*v3.Setting, error) {
		settingsETCD[s.Name] = s
		return s, nil
	}).AnyTimes()

	alreadyExistsCreateClient.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*v3.SettingList, error) {
		return &v3.SettingList{}, nil
	}).Times(1)

	settingsETCD = make(map[string]*v3.Setting)

	err = testSettingsProvider.SetAll(settingMap)
	assert.Nilf(t, err, "SetAll should not return an error if setting client's Create returns an AlreadyExists error."+
		" This is because it is assumed that if AlreadyExists is returned, than a different node in the setup created it.")
}

func populateTestCases() []*testCase {
	testCases := []*testCase{
		{
			description: "test an existing setting with val and empty source being reconfigured with env var uses the value from env var",
			envVar:      pointer.String("notempty"),
			newDefVal:   "abc",
			existingSetting: &v3.Setting{
				Value:   "somethingelse",
				Default: "abc",
			},
		},
		{
			description: "test creating a setting that doesn't exist yet creates the setting in kubernetes",
			newDefVal:   "abc",
		},
		{
			description: "test changing default of existing setting with a value properly updates the default but doesn't change value",
			newDefVal:   "newDef",
			existingSetting: &v3.Setting{
				Value:   "somethingelse",
				Default: "oldDef",
			},
		},
		{
			description: "test changing default of existing setting without a value properly update the default and nothing else",
			newDefVal:   "newDef",
			existingSetting: &v3.Setting{
				Default: "oldDef",
			},
		},
		{
			description: "test an existing setting with val and \"env\" source being reconfigured with env var updates value to the new env var value",
			newDefVal:   "abc",
			envVar:      pointer.String("notempty"),
			existingSetting: &v3.Setting{
				Value:   "somethingelse",
				Default: "abc",
				Source:  "env",
			},
		},
		{
			description: "test a setting that doesn't exist with val and \"env\" source being configured with env var creates setting with" +
				" env var value and \"env\" source",
			newDefVal: "abc",
			envVar:    pointer.String("notempty"),
		},
		{
			description: "test that setting an empty string value using an environment variable works when the env var was not used prior",
			newDefVal:   "abc",
			envVar:      pointer.String(""),
			existingSetting: &v3.Setting{
				Value:   "somethingelse",
				Default: "abc",
			},
		},
		{
			description: "test that setting an empty string value using an environment variable works when the env var was used prior.",
			newDefVal:   "abc",
			envVar:      pointer.String(""),
			existingSetting: &v3.Setting{
				Value:   "somethingelse",
				Default: "abc",
				Source:  "env",
			},
		},
	}

	for index, test := range testCases {
		settingName := fmt.Sprintf("test%d", index)
		newSetting := settings.NewSetting(settingName, test.newDefVal)
		test.newSetting = newSetting
		if test.existingSetting == nil {
			continue
		}
		test.existingSetting.Name = settingName
	}
	return testCases
}

func TestMarkSettingAsUnknownRetry(t *testing.T) {
	settingsETCD := map[string]v3.Setting{
		"unknown": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "unknown",
			},
			Value:   "unknown",
			Default: "unknown",
		},
	}

	client := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))

	client.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options metav1.GetOptions) (*v3.Setting, error) {
		val, ok := settingsETCD[name]
		if !ok {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		return &val, nil
	}).Times(1)

	var updateRun int
	client.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *v3.Setting) (*v3.Setting, error) {
		defer func() { updateRun++ }()

		if updateRun == 0 { // Fail the the first update to force retry.
			return nil, apierrors.NewConflict(schema.GroupResource{}, s.Name, fmt.Errorf("some error"))
		}

		settingsETCD[s.Name] = *s
		return s, nil
	}).Times(2)

	provider := settingsProvider{
		settings: client,
	}

	unknown := settingsETCD["unknown"]

	err := provider.markSettingAsUnknown(&unknown)
	assert.Nil(t, err)

	assert.NotNil(t, settingsETCD["unknown"].Labels)
	assert.Equal(t, settingsETCD["unknown"].Labels["cattle.io/unknown"], "true")
}
