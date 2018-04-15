package node

import (
	"context"
	"os"
	"strings"

	"github.com/docker/docker/client"
	"github.com/rancher/norman/types/slice"
	"github.com/sirupsen/logrus"
)

func TokenAndURL() (string, string, error) {
	return os.Getenv("CATTLE_TOKEN"), os.Getenv("CATTLE_SERVER"), nil
}

func Params() map[string]interface{} {
	roles := split(os.Getenv("CATTLE_ROLE"))
	params := map[string]interface{}{
		"customConfig": map[string]interface{}{
			"address":         os.Getenv("CATTLE_ADDRESS"),
			"internalAddress": os.Getenv("CATTLE_INTERNAL_ADDRESS"),
			"roles":           split(os.Getenv("CATTLE_ROLE")),
		},
		"etcd":              slice.ContainsString(roles, "etcd"),
		"controlPlane":      slice.ContainsString(roles, "controlplane"),
		"worker":            slice.ContainsString(roles, "worker"),
		"requestedHostname": os.Getenv("CATTLE_NODE_NAME"),
	}

	for k, v := range params {
		if m, ok := v.(map[string]string); ok {
			for k, v := range m {
				logrus.Infof("Option %s=%s", k, v)
			}
		} else {
			logrus.Infof("Option %s=%v", k, v)
		}
	}

	dclient, err := client.NewEnvClient()
	if err == nil {
		info, err := dclient.Info(context.Background())
		if err == nil {
			params["dockerInfo"] = info
		}
	}

	return map[string]interface{}{
		"node": params,
	}
}

func split(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 1 && result[0] == "" {
		return nil
	}
	return result
}
