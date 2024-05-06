package userretention

import (
	"fmt"
	"strings"
	"time"

	appsettings "github.com/rancher/rancher/pkg/settings"
)

// settings control user retention process.
type settings struct {
	disableAfter     time.Duration
	deleteAfter      time.Duration
	defaultLastLogin time.Time
	dryRun           bool
}

// ShouldDisable returns true if the user retention process should disable users.
func (s *settings) ShouldDisable() bool {
	return s.disableAfter != 0
}

// ShouldDelete returns true if the user retention process should delete users.
func (s *settings) ShouldDelete() bool {
	return s.deleteAfter != 0
}

// FormatDefaultLastLogin returns formatted value of the default last login.
func (s *settings) FormatDefaultLastLogin() string {
	if s.defaultLastLogin.IsZero() {
		return ""
	}

	return s.defaultLastLogin.Format(time.RFC3339)
}

// readSettings reads and parses user retention settings.
func readSettings() (settings, error) {
	var (
		err    error
		parsed settings
	)

	if value := appsettings.DisableInactiveUserAfter.Get(); value != "" {
		parsed.disableAfter, err = time.ParseDuration(value)
		if err != nil {
			return settings{}, fmt.Errorf("%s: %w", appsettings.DisableInactiveUserAfter.Name, err)
		}
	}

	if value := appsettings.DeleteInactiveUserAfter.Get(); value != "" {
		parsed.deleteAfter, err = time.ParseDuration(value)
		if err != nil {
			return settings{}, fmt.Errorf("%s: %w", appsettings.DeleteInactiveUserAfter.Name, err)
		}
	}

	if value := appsettings.UserLastLoginDefault.Get(); value != "" {
		parsed.defaultLastLogin, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return settings{}, fmt.Errorf("%s: %w", appsettings.UserLastLoginDefault.Name, err)
		}
	}

	parsed.dryRun = strings.EqualFold(appsettings.UserRetentionDryRun.Get(), "true")

	return parsed, nil
}
