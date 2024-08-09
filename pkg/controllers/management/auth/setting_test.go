package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSettingsSyncWithEmptyAzureGroupCacheSize(t *testing.T) {
	var azureGroupCacheSizeCalledTimes int
	controller := &SettingController{
		azureUpdateGroupCacheSize: func(_ string) error {
			azureGroupCacheSizeCalledTimes++
			return nil
		},
	}
	name := settings.AzureGroupCacheSize.Name
	t.Run(name, func(t *testing.T) {
		_, err := controller.sync(name, &v3.Setting{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Value:      "",
		})
		if err != nil {
			t.Fatal(err)
		}
		if want, got := 1, azureGroupCacheSizeCalledTimes; want != got {
			t.Fatalf("Expected azureGroupCacheSizeCalledTimes: %d got %d", want, got)
		}
	})
}

func TestSettingsSyncEnsureUserRetentionLabels(t *testing.T) {
	for _, name := range []string{
		settings.DisableInactiveUserAfter.Name,
		settings.DeleteInactiveUserAfter.Name,
		settings.UserLastLoginDefault.Name,
	} {
		t.Run(name, func(t *testing.T) {
			var ensureLabelsCalledTimes int
			controller := &SettingController{
				ensureUserRetentionLabels: func() error {
					ensureLabelsCalledTimes++
					return nil
				},
			}

			_, err := controller.sync(name, &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Value:      "1h",
			})
			if err != nil {
				t.Fatal(err)
			}

			if want, got := 1, ensureLabelsCalledTimes; want != got {
				t.Errorf("Expected ensureLabelsCalledTimes: %d got %d", want, got)
			}
		})
	}
}

func TestSettingsSyncScheduleUserRetention(t *testing.T) {
	var scheduleRetentionCalledTimes int
	controller := &SettingController{
		scheduleUserRetention: func(_ string) error {
			scheduleRetentionCalledTimes++
			return nil
		},
	}

	name := settings.UserRetentionCron.Name
	_, err := controller.sync(name, &v3.Setting{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Value:      "* * * * *",
	})
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 1, scheduleRetentionCalledTimes; want != got {
		t.Fatalf("Expected scheduleRetentionCalledTimes: %d got %d", want, got)
	}
}

func TestSettingsSyncWithProviderRefresh(t *testing.T) {
	// counter to ensure the providerrefresh are called during execution
	var providerRefreshCalledTimes int
	controller := &SettingController{
		providerRefreshCronTime: func(_ string) error {
			providerRefreshCalledTimes++
			return nil
		},
		providerRefreshMaxAge: func(_ string) error {
			providerRefreshCalledTimes++
			return nil
		},
	}

	for _, name := range []string{
		settings.AuthUserInfoResyncCron.Name,
		settings.AuthUserInfoMaxAgeSeconds.Name,
	} {
		t.Run(name, func(t *testing.T) {
			// reset the value after each test run
			defer func() {
				providerRefreshCalledTimes = 0
			}()

			_, err := controller.sync(name, &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Value:      "* * * * *",
			})
			if err != nil {
				t.Fatal(err)
			}
			if want, got := 1, providerRefreshCalledTimes; want != got {
				t.Fatalf("Expected providerRefreshCalledTimes: %d got %d", want, got)
			}
		})
	}
}
