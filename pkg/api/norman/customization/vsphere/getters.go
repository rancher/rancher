package vsphere

import (
	"context"
	"strings"

	"github.com/rancher/rancher/pkg/capr"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

// processSoapFinderGetters processes the vmPath that is passed in from the UI
// and contains the inventory path to the VM being cloned
// the format needs to match what machine uses (example below)
// vSphere args: cloneFrom:/datacenter/vm/username/sles15-sp3-template
// vSphere cmd: --vmwarevsphere-clone-from=/datacenter/vm/username/sles15-sp3-template
func processSoapFinderGetters(ctx context.Context, vmPath, fieldName string, cc *v1.Secret, dc string) ([]string, error) {
	finder, err := getSoapFinder(ctx, cc, dc)
	if err != nil {
		return nil, err
	}
	vm, err := finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, err
	}
	var data []string
	var d string
	switch fieldName {
	case "guest-os":
		d, err = getVirtualMachineGuestOS(ctx, vm)
		data = append(data, d)
	}
	return data, err
}

// getVirtualMachineGuestOS fetches vSphere VM object properties to get the guestId
func getVirtualMachineGuestOS(ctx context.Context, vm *object.VirtualMachine) (string, error) {
	var mvm mo.VirtualMachine

	refErr := vm.Properties(ctx, vm.Reference(), []string{"summary.config.guestId"}, &mvm)
	if refErr != nil {
		return "", refErr
	}
	guestOS := checkGuestID(mvm.Summary.Config.GuestId)

	return guestOS, nil
}

// checkGuestID parses a vSphere VM object's guestId properties and
// returns a simplified OS string which will default to linux
// if the guestId is not Windows
func checkGuestID(g string) string {
	var machineOS string
	switch {
	case strings.Contains(strings.ToLower(g), capr.WindowsMachineOS):
		machineOS = capr.WindowsMachineOS
	default:
		machineOS = capr.DefaultMachineOS
	}
	return machineOS
}
