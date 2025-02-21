package common

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Decode will decode to the output structure by creating a custom decoder
// that uses the stringToK8sTimeHookFunc to handle the metav1.Time field properly.
func Decode(input, output any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: stringToK8sTimeHookFunc(),
		Result:     output,
	})
	if err != nil {
		return fmt.Errorf("unable to create decoder for Config: %w", err)
	}
	err = decoder.Decode(input)
	if err != nil {
		return fmt.Errorf("unable to decode Config: %w", err)
	}
	return nil
}

// stringToTimeHookFunc returns a DecodeHookFunc that converts strings to metav1.Time.
func stringToK8sTimeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeOf(metav1.Time{}) {
			return data, nil
		}

		// Convert it by parsing
		stdTime, err := time.Parse(time.RFC3339, data.(string))
		return metav1.Time{Time: stdTime}, err
	}
}

// GetCommonUserExtraAttributes is a helper containing the default
// implementation of the `GetUserExtraAttributes` provider interface method.
func GetCommonUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	extras := make(map[string][]string)
	if userPrincipal.Name != "" {
		extras[UserAttributePrincipalID] = []string{userPrincipal.Name}
	}
	if userPrincipal.LoginName != "" {
		extras[UserAttributeUserName] = []string{userPrincipal.LoginName}
	}
	return extras
}
