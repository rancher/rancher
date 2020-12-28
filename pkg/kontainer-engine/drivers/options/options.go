package options

import "github.com/rancher/rancher/pkg/kontainer-engine/types"

func GetValueFromDriverOptions(driverOptions *types.DriverOptions, optionType string, keys ...string) interface{} {
	switch optionType {
	case types.IntType:
		for _, key := range keys {
			if value, ok := driverOptions.IntOptions[key]; ok {
				return value
			}
		}
		return int64(0)
	case types.IntPointerType:
		for _, key := range keys {
			if value, ok := driverOptions.IntOptions[key]; ok {
				return &value
			}
		}
		return nil
	case types.StringType:
		for _, key := range keys {
			if value, ok := driverOptions.StringOptions[key]; ok {
				return value
			}
		}
		return ""
	case types.BoolType:
		for _, key := range keys {
			if value, ok := driverOptions.BoolOptions[key]; ok {
				return value
			}
		}
		return false
	case types.BoolPointerType:
		for _, key := range keys {
			if value, ok := driverOptions.BoolOptions[key]; ok {
				return &value
			}
		}
		return nil
	case types.StringSliceType:
		for _, key := range keys {
			if value, ok := driverOptions.StringSliceOptions[key]; ok {
				return value
			}
		}
		return &types.StringSlice{}
	}
	return nil
}
