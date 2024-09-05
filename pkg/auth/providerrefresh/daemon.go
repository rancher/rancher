package providerrefresh

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/auth/settings"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
)

var (
	DefaultRefresher = &refresher{}
)

func StartRefreshDaemon(ctx context.Context, scaledContext *config.ScaledContext, mgmtContext *config.ManagementContext) {
	refreshCronTime := settings.AuthUserInfoResyncCron.Get()
	maxAge := settings.AuthUserInfoMaxAgeSeconds.Get()
	DefaultRefresher = &refresher{
		tokenLister:         mgmtContext.Management.Tokens("").Controller().Lister(),
		tokens:              mgmtContext.Management.Tokens(""),
		userLister:          mgmtContext.Management.Users("").Controller().Lister(),
		tokenMGR:            tokens.NewManager(ctx, scaledContext),
		userAttributes:      mgmtContext.Management.UserAttributes(""),
		userAttributeLister: mgmtContext.Management.UserAttributes("").Controller().Lister(),
		cron:                *cron.New(),
	}

	UpdateRefreshMaxAge(maxAge)
	UpdateRefreshCronTime(refreshCronTime)

}

// UpdateRefreshMaxAge updates the current refresh cron time with the one provided as input.
// Returns an error in case of failure.
func UpdateRefreshCronTime(refreshCronTime string) error {
	return DefaultRefresher.updateRefreshCronTime(refreshCronTime)
}

// this method is used just for testing purposes
func (ref *refresher) updateRefreshCronTime(refreshCronTime string) error {
	if refreshCronTime == "" {
		return fmt.Errorf("refresh cron time must be provided")
	}

	parsed, err := ParseCron(refreshCronTime)
	if err != nil {
		return fmt.Errorf("parsing error: %v", err)
	}

	ref.cron.Stop()
	ref.cron = *cron.New()

	if parsed != nil {
		job := cron.FuncJob(RefreshAllForCron)
		ref.cron.Schedule(parsed, job)
		ref.cron.Start()
	}
	return nil
}

// UpdateRefreshMaxAge parse the maxAge string given in input and set
// the ref.maxAge attribute to the equivalent time.Duration
// Returns an error in case of failure.
func UpdateRefreshMaxAge(maxAge string) error {
	return DefaultRefresher.updateRefreshMaxAge(maxAge)
}

// this method is used just for testing purposes
func (ref *refresher) updateRefreshMaxAge(maxAge string) error {
	if maxAge == "" {
		return fmt.Errorf("refresh max age must be provided")
	}

	ref.ensureMaxAgeUpToDate(maxAge)
	return nil
}

// RefreshAllForCron refreshes all the users crons.
func RefreshAllForCron() {
	DefaultRefresher.refreshAllForCron()
}

func (ref *refresher) refreshAllForCron() {
	logrus.Debug("Triggering auth refresh cron")
	ref.refreshAll(false)
}

func RefreshAttributes(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
	return DefaultRefresher.refreshUserAttributes(attribs)
}

func (ref *refresher) refreshUserAttributes(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
	logrus.Debugf("Starting refresh process for %v", attribs.Name)
	modified, err := ref.refreshAttributes(attribs)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Finished refresh process for %v", attribs.Name)
	modified.LastRefresh = time.Now().UTC().Format(time.RFC3339)
	modified.NeedsRefresh = false
	return modified, nil
}

func ParseMaxAge(setting string) (time.Duration, error) {
	durString := fmt.Sprintf("%vs", setting)
	dur, err := time.ParseDuration(durString)
	if err != nil {
		return 0, fmt.Errorf("error parsing auth refresh max age: %v", err)
	}
	return dur, nil
}

func ParseCron(setting string) (cron.Schedule, error) {
	if setting == "" {
		return nil, nil
	}
	schedule, err := cron.ParseStandard(setting)
	if err != nil {
		return nil, fmt.Errorf("error parsing auth refresh cron: %v", err)
	}
	return schedule, nil
}
