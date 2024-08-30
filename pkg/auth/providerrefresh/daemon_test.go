package providerrefresh

import (
	"reflect"
	"testing"

	"github.com/robfig/cron"
)

func Test_updateRefreshCronTime(t *testing.T) {
	ref := &refresher{}

	ref.cron = *cron.New()
	currentRefresh, err := cron.ParseStandard("0 0 * * *")
	if err != nil {
		t.Fatalf("error parsing cron time")
	}
	ref.cron.Schedule(currentRefresh, cron.FuncJob(func() {}))
	currentEntries := ref.cron.Entries()

	err = ref.updateRefreshCronTime("*/5 * * * *")
	if err != nil {
		t.Fatal(err)
	}

	newEntries := ref.cron.Entries()

	if reflect.DeepEqual(newEntries[0].Schedule, currentEntries[0].Schedule) {
		t.Fatalf("error: cron new entry should differ from the old one: %v\n%v", newEntries[0].Schedule, currentEntries[0].Schedule)
	}
}
