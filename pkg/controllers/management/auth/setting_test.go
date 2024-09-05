package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
