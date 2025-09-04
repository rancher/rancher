package nodedriver

import (
	"fmt"
	"net/rpc"
	"os"
	"os/user"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/rancher/machine/libmachine/drivers/plugin/localbinary"
	rpcdriver "github.com/rancher/machine/libmachine/drivers/rpc"
	cli "github.com/rancher/machine/libmachine/mcnflag"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/sirupsen/logrus"
)

const (
	FileToFieldAliasesAnno = "nodedriver.cattle.io/file-to-field-aliases"
)

func FlagToField(flag cli.Flag) (string, v32.Field, error) {
	field := v32.Field{
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
		field.Nullable = true
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
	var flags []cli.Flag

	logrus.Debugf("Starting binary %s", driver)
	finalDriverName := driver

	if os.Getenv("CATTLE_DEV_MODE") == "" {
		core := slices.Contains(localbinary.CoreDrivers, driver)
		if !core {
			fullName := fmt.Sprintf("%s%s", drivers.DockerMachineDriverPrefix, driver)
			finalDriverName = path.Join("/opt/drivers/management-state/bin", fullName)

			u, err := user.Lookup(jailer.JailUser)
			if err != nil {
				return nil, fmt.Errorf("error getting jailed user: %w", err)
			}
			g, err := user.LookupGroup(jailer.JailGroup)
			if err != nil {
				return nil, fmt.Errorf("error getting jailed group: %w", err)
			}

			defer func() {
				if err := os.Unsetenv(localbinary.PluginUID); err != nil {
					logrus.Warnf("Error unsetting env var: %v", err)
				}
				if err := os.Unsetenv(localbinary.PluginGID); err != nil {
					logrus.Warnf("Error unsetting env var: %v", err)
				}
			}()

			if err := os.Setenv(localbinary.PluginUID, u.Uid); err != nil {
				return nil, fmt.Errorf("error setting env var: %w", err)
			}
			if err := os.Setenv(localbinary.PluginGID, g.Gid); err != nil {
				return nil, fmt.Errorf("error setting env var: %w", err)
			}
		}
	}
	p, err := localbinary.NewPlugin(finalDriverName)
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

	if err := c.Call(".GetCreateFlags", struct{}{}, &flags); err != nil {
		return nil, fmt.Errorf("Error getting flags err=%v", err)
	}

	return flags, nil
}

// ParseKeyValueString parses a comma-separated list of "key:value" pairs into a map[string]string
func ParseKeyValueString(input string) map[string]string {
	result := map[string]string{}

	if strings.TrimSpace(input) == "" {
		logrus.Debugf("Empty input string")
		return result
	}

	pairs := strings.SplitSeq(input, ",")
	for pair := range pairs {
		keyVal := strings.Split(pair, ":")
		key := strings.TrimSpace(keyVal[0])
		value := strings.TrimSpace(keyVal[1])
		if len(keyVal) != 2 || key == "" {
			logrus.Errorf("failed to parse pair: %q (expected key:value)", pair)
			continue
		}
		result[key] = value
	}
	return result
}
