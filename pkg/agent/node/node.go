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
	token := os.Getenv("CATTLE_TOKEN")
	server := os.Getenv("CATTLE_SERVER")
	
	// ADD DEBUG: Log the token and server values
	logrus.Infof("DEBUG: TokenAndURL() - CATTLE_TOKEN: %s", token)
	logrus.Infof("DEBUG: TokenAndURL() - CATTLE_SERVER: %s", server)
	
	return token, server, nil
}

func Params() map[string]interface{} {
	labels := parseLabel(os.Getenv("CATTLE_NODE_LABEL"))
	taints := split(os.Getenv("CATTLE_NODE_TAINTS"))
	roles := split(os.Getenv("CATTLE_ROLE"))
	params := map[string]interface{}{
		"customConfig": map[string]interface{}{
			"address":         os.Getenv("CATTLE_ADDRESS"),
			"internalAddress": os.Getenv("CATTLE_INTERNAL_ADDRESS"),
			"roles":           split(os.Getenv("CATTLE_ROLE")),
			"label":           labels,
			"taints":          taints,
		},
		"etcd":              slice.ContainsString(roles, "etcd"),
		"controlPlane":      slice.ContainsString(roles, "controlplane"),
		"worker":            slice.ContainsString(roles, "worker"),
		"requestedHostname": os.Getenv("CATTLE_NODE_NAME"),
	}

	dclient, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		logrus.Errorf("Error getting docker client: %v", err)
	} else {
		defer dclient.Close()
		info, err := dclient.Info(context.Background())
		if err != nil {
			logrus.Errorf("Error getting docker info: %v", err)
		} else {
			params["dockerInfo"] = info
		}
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

	result := map[string]interface{}{
		"node": params,
	}
	
	// ADD DEBUG: Log the final result
	logrus.Infof("DEBUG: Node.Params() returning: %+v", result)
	
	return result
}

func parseLabel(v string) map[string]string {
	labels := map[string]string{}
	parts := strings.Split(v, ",")
	for _, part := range parts {
		if part == "" {
			continue
		}
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
