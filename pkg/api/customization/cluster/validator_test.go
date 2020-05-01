package cluster

import (
	"encoding/json"
	mgmtclient "github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	"testing"
)

const clusterSpecJSON = `
{
    "dockerRootDir": "/var/lib/docker",
    "enableClusterAlerting": false,
    "enableClusterMonitoring": false,
    "enableNetworkPolicy": false,
    "labels": {},
    "localClusterAuthEndpoint": {
        "enabled": true,
        "type": "localClusterAuthEndpoint"
    },
    "name": "testcluster",
    "rancherKubernetesEngineConfig": {
        "addonJobTimeout": 30,
        "authentication": {
            "strategy": "x509",
            "type": "authnConfig"
        },
        "dns": {
            "nodelocal": {
                "ip_address": "",
                "node_selector": null,
                "type": "nodelocal",
                "update_strategy": {}
            },
            "type": "dnsConfig"
        },
        "ignoreDockerVersion": true,
        "ingress": {
            "provider": "nginx",
            "type": "ingressConfig"
        },
        "kubernetesVersion": "v1.17.3-rancher1-2",
        "monitoring": {
            "provider": "metrics-server",
            "replicas": 1,
            "type": "monitoringConfig"
        },
        "network": {
            "mtu": 0,
            "options": {
                "flannel_backend_type": "vxlan"
            },
            "plugin": "canal",
            "type": "networkConfig"
        },
        "services": {
            "kubeApi": {
                "alwaysPullImages": false,
                "podSecurityPolicy": false,
                "serviceNodePortRange": "30000-32767",
                "type": "kubeAPIService"
            },
            "type": "rkeConfigServices"
        },
        "sshAgentAuth": false,
        "type": "rancherKubernetesEngineConfig",
        "upgradeStrategy": {
            "drain": true,
            "maxUnavailableControlplane": "1",
            "maxUnavailableUnit": "percentage",
            "maxUnavailableWorker": "10%",
            "nodeDrainInput": {
                "deleteLocalData": false,
                "force": false,
                "gracePeriod": null,
                "ignoreDaemonSets": true,
                "timeout": 60,
                "type": "nodeDrainInput",
                "unlimitedTimeout": false,
                "usePodGracePeriod": true
            }
        }
    },
    "scheduledClusterScan": {
        "enabled": true,
        "scanConfig": {
            "cisScanConfig": {
                "failuresOnly": false,
                "overrideBenchmarkVersion": "rke-cis-1.4",
                "profile": "permissive",
                "skip": null
            }
        },
        "scheduleConfig": {
            "cronSchedule": ""
        }
    },
    "type": "cluster",
    "windowsPreferedCluster": false
}
`

const clusterTemplateSpecJSON = `
{
    "dockerRootDir": "/var/lib/docker",
    "enableClusterAlerting": false,
    "enableClusterMonitoring": false,
    "enableNetworkPolicy": false,
    "labels": {},
    "clusterTemplateRevisionId": "cattle-global-data:ctr-xxxxx",
    "name": "testclusterfromtemplate",
    "rancherKubernetesEngineConfig": {
    },
    "scheduledClusterScan": {
        "enabled": true,
        "scanConfig": {
            "cisScanConfig": {
                "failuresOnly": false,
                "overrideBenchmarkVersion": "rke-cis-1.4",
                "profile": "permissive",
                "skip": null
            }
        },
        "scheduleConfig": {
            "cronSchedule": ""
        }
    },
    "type": "cluster"
}
`

func TestValidateScheduledClusterScan(t *testing.T) {
	var clusterSpec mgmtclient.Cluster
	err := json.Unmarshal([]byte(clusterSpecJSON), &clusterSpec)
	if err != nil {
		logrus.Errorf("error unmarshaling clusterspec: %v", err)
		t.FailNow()
	}

	clusterSpec.ScheduledClusterScan.ScheduleConfig.Retention = -1
	err = validateScheduledClusterScan(&clusterSpec)
	if err == nil {
		logrus.Errorf("expected error")
		t.FailNow()
	}
	clusterSpec.ScheduledClusterScan.ScheduleConfig.Retention = 3

	clusterSpec.ScheduledClusterScan.ScheduleConfig.CronSchedule = "junk"
	err = validateScheduledClusterScan(&clusterSpec)
	if err == nil {
		logrus.Errorf("expected error")
		t.FailNow()
	}

	everyHour := "0 */1 * * *"
	clusterSpec.ScheduledClusterScan.ScheduleConfig.CronSchedule = everyHour
	err = validateScheduledClusterScan(&clusterSpec)
	if err != nil {
		logrus.Errorf("not expecting error, got: %v", err)
		t.FailNow()
	}

	everyMinute := "* * * * *"
	clusterSpec.ScheduledClusterScan.ScheduleConfig.CronSchedule = everyMinute
	err = validateScheduledClusterScan(&clusterSpec)
	if err == nil {
		logrus.Errorf("expected error")
		t.FailNow()
	}

	clusterSpec.ScheduledClusterScan.ScanConfig.CisScanConfig.DebugMaster = true
	err = validateScheduledClusterScan(&clusterSpec)
	if err != nil {
		logrus.Errorf("not expecting error, got: %v", err)
		t.FailNow()
	}
}

func TestClusterTemplateValidateScheduledClusterScan(t *testing.T) {
	var clusterTemplateSpec mgmtclient.Cluster
	err := json.Unmarshal([]byte(clusterTemplateSpecJSON), &clusterTemplateSpec)
	if err != nil {
		logrus.Errorf("error unmarshaling clusterspec: %v", err)
		t.FailNow()
	}

	err = validateScheduledClusterScan(&clusterTemplateSpec)
	if err != nil {
		logrus.Errorf("not expecting error, got: %v", err)
		t.FailNow()
	}
}
