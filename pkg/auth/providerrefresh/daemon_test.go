package providerrefresh

import (
	"context"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementFakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/robfig/cron"
)

func TestUpdateRefreshCronTime(t *testing.T) {
	ctx := context.Background()
	scaledContext := &config.ScaledContext{}

	// filling needed mocks to simulate mgmtContext parameter
	// on StartRefreshDaemon
	mgmtContextMock := &managementFakes.InterfaceMock{}
	tockensInterfaceMock := &managementFakes.TokenInterfaceMock{}
	tokensControllerMock := &managementFakes.TokenControllerMock{}
	tokensListerMock := &managementFakes.TokenListerMock{}
	userInterfaceMock := &managementFakes.UserInterfaceMock{}
	userControllerMock := &managementFakes.UserControllerMock{}
	userListerMock := &managementFakes.UserListerMock{}
	userAttributesInterfaceMock := &managementFakes.UserAttributeInterfaceMock{}
	userAttributesControllerMock := &managementFakes.UserAttributeControllerMock{}

	// mgmtContext.Management.Tokens("").Controller().Lister()
	mgmtContextMock.TokensFunc = func(namespace string) v3.TokenInterface {
		return tockensInterfaceMock
	}
	tockensInterfaceMock.ControllerFunc = func() v3.TokenController {
		return tokensControllerMock
	}
	tokensControllerMock.ListerFunc = func() v3.TokenLister {
		return tokensListerMock
	}
	// mgmtContext.Management.Users("").Controller().Lister()
	mgmtContextMock.UsersFunc = func(namespace string) v3.UserInterface {
		return userInterfaceMock
	}
	userInterfaceMock.ControllerFunc = func() v3.UserController {
		return userControllerMock
	}
	userControllerMock.ListerFunc = func() v3.UserLister {
		return userListerMock
	}
	// mgmtContext.Management.UserAttributes("").Controller().Lister()
	mgmtContextMock.UserAttributesFunc = func(namespace string) v3.UserAttributeInterface {
		return userAttributesInterfaceMock
	}
	userAttributesInterfaceMock.ControllerFunc = func() v3.UserAttributeController {
		return userAttributesControllerMock
	}

	mgmtContext := &config.ManagementContext{
		Management: mgmtContextMock,
	}

	StartRefreshDaemon(ctx, scaledContext, mgmtContext)

	type args struct {
		refreshCronTime string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "first test",
			args: args{
				refreshCronTime: "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateRefreshCronTime(tt.args.refreshCronTime); (err != nil) != tt.wantErr {
				t.Errorf("UpdateRefreshCronTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

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
