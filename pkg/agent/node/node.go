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
	labels := parseLabel(os.Getenv("CATTLE_NODE_LABEL"))
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
		"labels":            labels,
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

func parseLabel(v string) map[string]string {
	labels := map[string]string{}
	parts := strings.Split(v, ",")
	for _, part := range parts {
		kvs := strings.SplitN(part, "=", 2)
		if len(kvs) == 2 {
			labels[kvs[0]] = kvs[1]
		} else if len(kvs) == 1 {
			labels[kvs[0]] = ""
		} else {
			logrus.Warnf("Invalid label format %v.", part)
		}
	}
	return labels
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
