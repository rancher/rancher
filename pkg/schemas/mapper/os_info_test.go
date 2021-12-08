package mapper

import (
	"reflect"
	"testing"
)

func Test_OsInfo(t *testing.T) {
	mapper := OSInfo{}

	tests := []struct {
		internal map[string]interface{}
		wantInfo map[string]interface{}
	}{
		{
			internal: map[string]interface{}{
				"capacity": map[string]interface{}{
					"cpu":    "2",
					"memory": "123456Ki",
				},
			},
			wantInfo: map[string]interface{}{
				"cpu": map[string]interface{}{
					"count": int64(2),
				},
				"memory": map[string]interface{}{
					"memTotalKiB": int64(123456),
				},
				"os": map[string]interface{}{
					"dockerVersion":   "",
					"kernelVersion":   nil,
					"operatingSystem": nil,
				},
				"kubernetes": map[string]interface{}{
					"kubeletVersion":   nil,
					"kubeProxyVersion": nil,
				},
			},
		},
		{
			internal: map[string]interface{}{
				"capacity": map[string]interface{}{
					"cpu":    "1M",
					"memory": "123456Ti",
				},
			},
			wantInfo: map[string]interface{}{
				"cpu": map[string]interface{}{
					"count": int64(1000000),
				},
				"memory": map[string]interface{}{
					"memTotalKiB": int64(132559870623744),
				},
				"os": map[string]interface{}{
					"dockerVersion":   "",
					"kernelVersion":   nil,
					"operatingSystem": nil,
				},
				"kubernetes": map[string]interface{}{
					"kubeletVersion":   nil,
					"kubeProxyVersion": nil,
				},
			},
		},
	}

	for _, tt := range tests {
		mapper.FromInternal(tt.internal)
		if !reflect.DeepEqual(tt.wantInfo, tt.internal["info"]) {
			t.Fatal("os info does not match after mapping")
		}
	}
}
