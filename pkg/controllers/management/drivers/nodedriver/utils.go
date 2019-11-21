package nodedriver

import (
	"fmt"
	"net/rpc"
	"reflect"
	"strconv"
	"strings"

	"github.com/rancher/machine/libmachine/drivers/plugin/localbinary"
	rpcdriver "github.com/rancher/machine/libmachine/drivers/rpc"
	cli "github.com/rancher/machine/libmachine/mcnflag"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func FlagToField(flag cli.Flag) (string, v3.Field, error) {
	field := v3.Field{
		Create: true,
		Update: true,
		Type:   "string",
	}

	name, err := ToLowerCamelCase(flag.String())
	if err != nil {
		return name, field, err
	}

	switch v := flag.(type) {
	case *cli.StringFlag:
		field.Description = v.Usage
		field.Default.StringValue = v.Value
	case *cli.IntFlag:
		// This will make the int flag appear as a string field in the rancher API, but we are doing this to maintain
		// backward compatibility, at least until we fix a bug that prevents nodeDriver schemas from updating upon
		// a Rancher upgrade
		field.Description = v.Usage
		field.Default.StringValue = strconv.Itoa(v.Value)
	case *cli.BoolFlag:
		field.Type = "boolean"
		field.Description = v.Usage
	case *cli.StringSliceFlag:
		field.Type = "array[string]"
		field.Description = v.Usage
		field.Default.StringSliceValue = v.Value
	case *BoolPointerFlag:
		field.Type = "boolean"
		field.Description = v.Usage
	default:
		return name, field, fmt.Errorf("unknown type of flag %v: %v", flag, reflect.TypeOf(flag))
	}

	return name, field, nil
}

func ToLowerCamelCase(nodeFlagName string) (string, error) {
	parts := strings.SplitN(nodeFlagName, "-", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("parameter %s does not follow expected naming convention [DRIVER]-[FLAG-NAME]", nodeFlagName)
	}
	flagNameParts := strings.Split(parts[1], "-")
	flagName := flagNameParts[0]
	for _, flagNamePart := range flagNameParts[1:] {
		flagName = flagName + strings.ToUpper(flagNamePart[:1]) + flagNamePart[1:]
	}
	return flagName, nil
}

func getCreateFlagsForDriver(driver string) ([]cli.Flag, error) {
	logrus.Debug("Starting binary ", driver)
	p, err := localbinary.NewPlugin(driver)
	if err != nil {
		return nil, err
	}
	go func() {
		err := p.Serve()
		if err != nil {
			logrus.Debugf("Error serving plugin server for driver=%s, err=%v", driver, err)
		}
	}()
	defer p.Close()
	addr, err := p.Address()
	if err != nil {
		return nil, err
	}

	rpcclient, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("Error dialing to plugin server's address(%v), err=%v", addr, err)
	}
	defer rpcclient.Close()

	c := rpcdriver.NewInternalClient(rpcclient)

	var flags []cli.Flag

	if err := c.Call(".GetCreateFlags", struct{}{}, &flags); err != nil {
		return nil, fmt.Errorf("Error getting flags err=%v", err)
	}

	return flags, nil
}
