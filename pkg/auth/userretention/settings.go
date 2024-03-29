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
func (c *settings) ShouldDisable() bool {
	return c.disableAfter != 0
}

// ShouldDelete returns true if the user retention process should delete users.
func (c *settings) ShouldDelete() bool {
	return c.deleteAfter != 0
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
