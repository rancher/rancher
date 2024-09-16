package settings

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
)

type testCase struct {
	description     string
	envVar          *string
	newDefVal       string
	newSetting      settings.Setting
	existingSetting *v3.Setting
}

func TestSetAll(t *testing.T) {
	t.Skip()
	client := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	provider := settingsProvider{
		settings: client,
	}

	store := make(map[string]v3.Setting)
	get := func(name string, options metav1.GetOptions) (*v3.Setting, error) {
		val, ok := store[name]
		if !ok {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		return &val, nil
	}
	set := func(setting *v3.Setting) (*v3.Setting, error) {
		store[setting.Name] = *setting
		return setting, nil
	}

	client.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
	client.EXPECT().Create(gomock.Any()).DoAndReturn(set).AnyTimes()
	client.EXPECT().Update(gomock.Any()).DoAndReturn(set).AnyTimes()
	client.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*v3.SettingList, error) {
		var items []v3.Setting
		for _, setting := range store {
			items = append(items, setting)
		}

		return &v3.SettingList{Items: items}, nil
	}).AnyTimes()
	client.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, opts *metav1.DeleteOptions) error {
		delete(store, name)
		return nil
	}).Times(1)

	settingMap := make(map[string]settings.Setting)
	testCases := populateTestCases()
	for _, test := range testCases {
		settingMap[test.newSetting.Name] = test.newSetting
		if test.envVar != nil {
			envKey := settings.GetEnvKey(test.newSetting.Name)
			os.Setenv(envKey, *test.envVar)
			defer os.Unsetenv(envKey)
		}

		if test.existingSetting != nil {
			store[test.newSetting.Name] = *test.existingSetting
		}
	}

	store["unknown"] = v3.Setting{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unknown",
		},
		Value:   "unknown",
		Default: "unknown",
	}

	err := provider.SetAll(settingMap)
	assert.Nil(t, err, "set all should not return an error")

	for _, test := range testCases {
		finalSetting, err := provider.settings.Get(test.newSetting.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		fallbackValue := provider.fallback[test.newSetting.Name]
		failMsg := fmt.Sprintf("test case failed [%s]: %s", test.newSetting.Name, test.description)
		fallbackFailMsg := fmt.Sprintf("test case failed [%s]: fallback value not properly set", test.newSetting.Name)

		// Updating setting in kubernetes should have the default from newSetting.
		assert.Equal(t, finalSetting.Default, test.newSetting.Default, failMsg)

		// If the value is configured by an environment variable, then the source should be "env", otherwise it should be empty.
		assert.True(t, finalSetting.Source == "env" == (test.envVar != nil), failMsg)

		var expectedFallbackVal string
		if test.envVar != nil {
			// Environment variable takes precedence of everything. Setting's value should match as long as it was set.
			assert.Equal(t, *test.envVar, finalSetting.Value, failMsg)
			expectedFallbackVal = *test.envVar
		} else if test.existingSetting != nil {
			expectedFallbackVal = test.existingSetting.Value
			assert.Equal(t, test.existingSetting.Value, finalSetting.Value, failMsg)
		} else {
			assert.Equal(t, "", finalSetting.Value, failMsg)
		}

		if expectedFallbackVal == "" {
			// Fallback value should be equal to default if value is empty. This is how clients of the settings provider
			// evaluate the effective value of the setting.
			expectedFallbackVal = test.newSetting.Default
		}

		assert.Equal(t, expectedFallbackVal, fallbackValue, fallbackFailMsg)
	}

	assert.NotContains(t, store, "unknown")

	// Make sure the store doesn't contain any settings with the unknown label.
	const unknownSettingLabelKey = "cattle.io/unknown"
	for _, setting := range store {
		assert.NotContains(t, setting.Labels, unknownSettingLabelKey)
	}

	// Test when setting client's Create method fails.
	cannotCreateClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	provider = settingsProvider{
		settings: cannotCreateClient,
	}
	cannotCreateClient.EXPECT().List(gomock.Any()).Return(new(v3.SettingList), nil).AnyTimes()
	cannotCreateClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
	cannotCreateClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(setting *v3.Setting) (*v3.Setting, error) {
		return nil, apierrors.NewServiceUnavailable("some error")
	}).AnyTimes()
	cannotCreateClient.EXPECT().Update(gomock.Any()).DoAndReturn(set).AnyTimes()

	store = make(map[string]v3.Setting)
	err = provider.SetAll(settingMap)
	assert.NotNilf(t, err, "SetAll should return an error if setting client's Create returns an error that is IsAlreadyExists.")

	cannotUpdateClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	provider = settingsProvider{
		settings: cannotCreateClient,
	}

	// Test when setting client's Update method fails.
	cannotUpdateClient.EXPECT().List(gomock.Any()).Return(new(v3.SettingList), nil).AnyTimes()
	cannotUpdateClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
	cannotUpdateClient.EXPECT().Create(gomock.Any()).DoAndReturn(set).AnyTimes()
	cannotUpdateClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *v3.Setting) (*v3.Setting, error) {
		return nil, apierrors.NewServiceUnavailable("some error")
	}).AnyTimes()

	store = make(map[string]v3.Setting)

	err = provider.SetAll(settingMap)
	assert.NotNilf(t, err, "SetAll should return an error if setting client's Update returns an error.")

	// Test when client's List method fails.
	cannotListClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	provider = settingsProvider{
		settings: cannotListClient,
	}
	cannotListClient.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*v3.SettingList, error) {
		return nil, apierrors.NewInternalError(errors.New("some error"))
	}).Times(1)

	err = provider.SetAll(settingMap)
	assert.Error(t, err, "SetAll should return an error if setting client's List returns an error.")

	// Test when setting client's Update method fails with AlreadyExists error.
	alreadyExistsCreateClient := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
	provider = settingsProvider{
		settings: alreadyExistsCreateClient,
	}

	alreadyExistsCreateClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
	alreadyExistsCreateClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(setting *v3.Setting) (*v3.Setting, error) {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "some error")
	}).AnyTimes()
	alreadyExistsCreateClient.EXPECT().Update(gomock.Any()).DoAndReturn(set).AnyTimes()
	alreadyExistsCreateClient.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*v3.SettingList, error) {
		return &v3.SettingList{}, nil
	}).AnyTimes()

	store = make(map[string]v3.Setting)

	err = provider.SetAll(settingMap)
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

func TestSetAllWithDefaultOnUpgrade(t *testing.T) {
	t.Skip()
	t.Parallel()
	t.Run("setting gets regular default on fresh install", func(t *testing.T) {
		t.Parallel()
		client := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
		provider := settingsProvider{
			settings: client,
		}
		store := make(map[string]v3.Setting)
		get, set, list := storeOperations(store)

		client.EXPECT().List(gomock.Any()).DoAndReturn(list).AnyTimes()
		client.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
		client.EXPECT().Create(gomock.Any()).DoAndReturn(set).AnyTimes()

		values := map[string]settings.Setting{
			"tls-agent-mode": {
				Name:             "tls-agent-mode",
				Default:          "strict",
				DefaultOnUpgrade: "system-store",
			},
		}
		err := provider.SetAll(values)
		require.NoError(t, err)
		s, err := get("tls-agent-mode", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "strict", s.Default)
	})

	t.Run("setting exists and keeps its default on upgrade having the special upgrade default", func(t *testing.T) {
		t.Parallel()
		client := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
		provider := settingsProvider{
			settings: client,
		}
		store := map[string]v3.Setting{
			"tls-agent-mode": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "tls-agent-mode",
				},
				Value:   "",
				Default: "system-store",
			},
		}
		get, set, list := storeOperations(store)

		client.EXPECT().List(gomock.Any()).DoAndReturn(list).AnyTimes()
		client.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
		client.EXPECT().Update(gomock.Any()).DoAndReturn(set).AnyTimes()

		values := map[string]settings.Setting{
			"tls-agent-mode": settings.NewSetting("tls-agent-mode", "strict").WithDefaultOnUpgrade("system-store"),
		}
		err := provider.SetAll(values)
		require.NoError(t, err)
		s, err := get("tls-agent-mode", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "system-store", s.Default)
	})

	t.Run("setting exists and keeps its default on upgrade in absence of an upgrade default value", func(t *testing.T) {
		t.Parallel()
		client := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
		provider := settingsProvider{
			settings: client,
		}
		store := map[string]v3.Setting{
			"some-setting": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-setting",
				},
				Default: "some-default",
			},
		}
		get, set, list := storeOperations(store)

		client.EXPECT().List(gomock.Any()).DoAndReturn(list).AnyTimes()
		client.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
		client.EXPECT().Update(gomock.Any()).DoAndReturn(set).AnyTimes()

		values := map[string]settings.Setting{
			"some-setting": settings.NewSetting("some-setting", "some-default"),
		}
		err := provider.SetAll(values)
		require.NoError(t, err)
		s, err := get("some-setting", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "some-default", s.Default)
	})

	t.Run("setting exists and gets its new default on upgrade in absence of an upgrade default value", func(t *testing.T) {
		t.Parallel()
		client := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
		provider := settingsProvider{
			settings: client,
		}
		store := map[string]v3.Setting{
			"some-setting": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-setting",
				},
				Default: "some-default",
			},
		}
		get, set, list := storeOperations(store)

		client.EXPECT().List(gomock.Any()).DoAndReturn(list).AnyTimes()
		client.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
		client.EXPECT().Update(gomock.Any()).DoAndReturn(set).AnyTimes()

		values := map[string]settings.Setting{
			"some-setting": settings.NewSetting("some-setting", "some-new-default"),
		}
		err := provider.SetAll(values)
		require.NoError(t, err)
		s, err := get("some-setting", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "some-new-default", s.Default)
	})

	t.Run("setting does not exist and gets a special default on upgrade value on non fresh install", func(t *testing.T) {
		t.Parallel()
		client := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](gomock.NewController(t))
		provider := settingsProvider{
			settings: client,
		}
		store := map[string]v3.Setting{
			"some-setting": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-setting",
				},
				Default: "some-default",
			},
		}
		get, set, list := storeOperations(store)

		client.EXPECT().List(gomock.Any()).DoAndReturn(list).AnyTimes()
		client.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
		client.EXPECT().Create(gomock.Any()).DoAndReturn(set).AnyTimes()

		values := map[string]settings.Setting{
			"some-setting":   settings.NewSetting("some-setting", "some-default"),
			"tls-agent-mode": settings.NewSetting("tls-agent-mode", "strict").WithDefaultOnUpgrade("system-store"),
		}
		err := provider.SetAll(values)
		require.NoError(t, err)
		s, err := get("tls-agent-mode", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "system-store", s.Default)
	})
}

func storeOperations(store map[string]v3.Setting) (get, set, list) {
	get := func(name string, opts metav1.GetOptions) (*v3.Setting, error) {
		val, ok := store[name]
		if !ok {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		return &val, nil
	}
	set := func(setting *v3.Setting) (*v3.Setting, error) {
		store[setting.Name] = *setting
		return setting, nil
	}
	list := func(opts metav1.ListOptions) (*v3.SettingList, error) {
		items := make([]v3.Setting, 0, len(store))
		for _, setting := range store {
			items = append(items, setting)
		}
		return &v3.SettingList{Items: items}, nil
	}
	return get, set, list
}

type get func(name string, opts metav1.GetOptions) (*v3.Setting, error)
type set func(setting *v3.Setting) (*v3.Setting, error)
type list func(opts metav1.ListOptions) (*v3.SettingList, error)
