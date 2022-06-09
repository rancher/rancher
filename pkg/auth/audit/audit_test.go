package audit

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/data/management"
)

func Test_concealSensitiveData(t *testing.T) {
	r, err := constructKeyConcealRegex()
	if err != nil {
		t.Fatalf("failed compiling sanitizing regex: %v", err)
	}
	a := auditLog{
		log:                nil,
		writer:             nil,
		reqBody:            nil,
		keysToConcealRegex: r,
	}

	machineDataInput, machineDataWant := []byte("{"), []byte("{")
	for _, v := range management.DriverData {
		for key, value := range v {
			if strings.HasPrefix(key, "optional") {
				continue
			}
			public := strings.HasPrefix(key, "public")
			for _, item := range value {
				machineDataInput = append(machineDataInput, []byte("\""+item+"\" : \"fake_"+item+"\",")...)
				if public {
					machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \"fake_"+item+"\",")...)
				} else {
					machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \""+redacted+"\",")...)
				}
			}
		}
	}
	machineDataInput[len(machineDataInput)-1] = byte('}')
	machineDataWant[len(machineDataWant)-1] = byte('}')

	tests := []struct {
		name  string
		uri   string
		input []byte
		want  []byte
	}{
		{
			name:  "password entry",
			input: []byte(`{"password": "fake_password", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			name:  "Password entry",
			input: []byte(`{"Password": "fake_password", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			name:  "password entry no space",
			input: []byte(`{"password":"whatever you want", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"password":"%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "Password entry no space",
			input: []byte(`{"Password":"A whole bunch of \"\"}{()","user":"fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"Password":"%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "currentPassword entry",
			input: []byte(`{"currentPassword": "something super secret", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"currentPassword": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "newPassword entry",
			input: []byte(`{"newPassword": "don't share this", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"newPassword": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "Multiple password entries",
			input: []byte(`{"currentPassword": "fake_password", "newPassword": "new_fake_password", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"currentPassword": "%s", "newPassword": "%[1]s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "No password entries",
			input: []byte(`{"user": "fake_user", "user_info": "some information about the user", "request_info": "some info about the request"}`),
			want:  []byte(`{"user": "fake_user", "user_info": "some information about the user", "request_info": "some info about the request"}`),
		},
		{
			name:  "Strategic password examples",
			input: []byte(`{"anotherPassword":"\"password\"","currentPassword":"password\":","newPassword":"newPassword\\\":","shortPassword":"'","user":"fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"anotherPassword":"%s","currentPassword":"%[1]s","newPassword":"%[1]s","shortPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},
		{
			name:  "Token entry",
			input: []byte(`{"accessToken": "fake_access_token", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"accessToken": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "With public fields",
			input: []byte(`{"accessKey": "fake_access_key", "secretKey": "fake_secret_key", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"accessKey": "fake_access_key", "secretKey": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "With secret data",
			input: []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			want:  []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"%s", "bar": "%[1]s"}, "accessToken" : "%[1]s"}`, redacted)),
			uri:   "/secrets",
		},
		{
			name:  "With secret data and wrong URI",
			input: []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			want:  []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "%s"}`, redacted)),
			uri:   "/not-secret",
		},
		{
			name:  "With nested sensitive information",
			input: []byte(`{"sensitiveData": {"accessToken": "fake_access_token", "user": "fake_user"}}`),
			want:  []byte(fmt.Sprintf(`{"sensitiveData": {"accessToken": "%s", "user": "fake_user"}}`, redacted)),
		},
		{
			name:  "With all machine driver fields",
			input: machineDataInput,
			want:  machineDataWant,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var want map[string]interface{}
			if err := json.Unmarshal(tt.want, &want); err != nil {
				t.Errorf("error unmarshaling: %v", err)
			}
			got := a.concealSensitiveData(tt.uri, tt.input)
			var gotMap map[string]interface{}
			if err := json.Unmarshal(got, &gotMap); err != nil {
				t.Errorf("error unmarshaling: %v", err)
			}
			if !reflect.DeepEqual(want, gotMap) {
				t.Errorf("concealSensitiveData() = %s, want %s", got, tt.want)
			}
		})
	}
}
