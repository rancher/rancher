package userretention

import (
	"reflect"
	"testing"
	"time"

	appsettings "github.com/rancher/rancher/pkg/settings"
)

func TestSettingsShouldDisable(t *testing.T) {
	tests := []struct {
		disableAfter  time.Duration
		shouldDisable bool
	}{
		{},
		{
			disableAfter:  time.Hour,
			shouldDisable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.disableAfter.String(), func(t *testing.T) {
			settings := settings{disableAfter: tt.disableAfter}

			if want, got := tt.shouldDisable, settings.ShouldDisable(); want != got {
				t.Errorf("Expected %t, got %t", want, got)
			}
		})
	}
}

func TestSettingsShouldDelete(t *testing.T) {
	tests := []struct {
		deleteAfter  time.Duration
		shouldDelete bool
	}{
		{},
		{
			deleteAfter:  time.Hour,
			shouldDelete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.deleteAfter.String(), func(t *testing.T) {
			settings := settings{deleteAfter: tt.deleteAfter}

			if want, got := tt.shouldDelete, settings.ShouldDelete(); want != got {
				t.Errorf("Expected %t, got %t", want, got)
			}
		})
	}
}

func TestReadSettings(t *testing.T) {
	tests := []struct {
		desc                     string
		disableInactiveUserAfter string
		deleteInactiveUserAfter  string
		userLastLoginDefault     string
		userRetentionDryRun      string
		parsed                   settings
		shouldErr                bool
	}{
		{
			desc: "all settings are empty",
		},
		{
			desc:                     "disableInactiveUserAfter is 0",
			disableInactiveUserAfter: "0",
		},
		{
			desc:                     "disableInactiveUserAfter is set",
			disableInactiveUserAfter: "1h30m30s",
			parsed: settings{
				disableAfter: time.Hour + 30*time.Minute + 30*time.Second,
			},
		},
		{
			desc:                    "deleteInactiveUserAfter is 0",
			deleteInactiveUserAfter: "0",
		},
		{
			desc:                    "deleteInactiveUserAfter is set",
			deleteInactiveUserAfter: "1h30m30s",
			parsed: settings{
				deleteAfter: time.Hour + 30*time.Minute + 30*time.Second,
			},
		},
		{
			desc:                 "userLastLoginDefault is set",
			userLastLoginDefault: "2024-03-15T12:00:00Z",
			parsed: settings{
				defaultLastLogin: time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			desc:                "userRetentionDryRun is true",
			userRetentionDryRun: "true",
			parsed: settings{
				dryRun: true,
			},
		},
		{
			desc:                "userRetentionDryRun is True",
			userRetentionDryRun: "true",
			parsed: settings{
				dryRun: true,
			},
		},
		{
			desc:                "userRetentionDryRun is false",
			userRetentionDryRun: "false",
		},
		{
			desc:                "userRetentionDryRun is neither true nor false",
			userRetentionDryRun: "foo",
		},
		{
			desc:                     "all settings are set",
			disableInactiveUserAfter: "1h30m30s",
			deleteInactiveUserAfter:  "1h30m30s",
			userLastLoginDefault:     "2024-03-15T12:00:00Z",
			userRetentionDryRun:      "true",
			parsed: settings{
				disableAfter:     time.Hour + 30*time.Minute + 30*time.Second,
				deleteAfter:      time.Hour + 30*time.Minute + 30*time.Second,
				defaultLastLogin: time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
				dryRun:           true,
			},
		},
		{
			desc:                     "disableInactiveUserAfter is invalid",
			disableInactiveUserAfter: "foo",
			shouldErr:                true,
		},
		{
			desc:                    "deleteInactiveUserAfter is invalid",
			deleteInactiveUserAfter: "foo",
			shouldErr:               true,
		},
		{
			desc:                 "userLastLoginDefault is invalid",
			userLastLoginDefault: "foo",
			shouldErr:            true,
		},
	}

	for _, tt := range tests {
		// Note: subtests can't be run in parralel because we're modifying global settings.
		t.Run(tt.desc, func(t *testing.T) {
			// Preserve the original settings.
			disableInactiveUserAfter := appsettings.DisableInactiveUserAfter.Get()
			deleteInactiveUserAfter := appsettings.DeleteInactiveUserAfter.Get()
			userLastLoginDefault := appsettings.UserLastLoginDefault.Get()
			userRetentionDryRun := appsettings.UserRetentionDryRun.Get()
			defer func() {
				// Restore the settings.
				appsettings.DisableInactiveUserAfter.Set(disableInactiveUserAfter)
				appsettings.DeleteInactiveUserAfter.Set(deleteInactiveUserAfter)
				appsettings.UserLastLoginDefault.Set(userLastLoginDefault)
				appsettings.UserRetentionDryRun.Set(userRetentionDryRun)
			}()

			appsettings.DisableInactiveUserAfter.Set(tt.disableInactiveUserAfter)
			appsettings.DeleteInactiveUserAfter.Set(tt.deleteInactiveUserAfter)
			appsettings.UserLastLoginDefault.Set(tt.userLastLoginDefault)
			appsettings.UserRetentionDryRun.Set(tt.userRetentionDryRun)

			parsed, err := readSettings()
			if err != nil {
				if !tt.shouldErr {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				// In the case of an expected error all the retention settings should have zero values.
			}

			if want, got := tt.parsed, parsed; !reflect.DeepEqual(want, got) {
				t.Errorf("Expected \n%+v\ngot\n%+v", want, got)
			}
		})
	}
}
