package settings

const (
	// ClusterAgentAffinity used to be hardcoded in the agent deployment template but is now defined here as the
	// single source of truth.
	ClusterAgentAffinity = `{
  "nodeAffinity": {
    "requiredDuringSchedulingIgnoredDuringExecution": {
      "nodeSelectorTerms": [
        {
          "matchExpressions": [
            {
              "key": "beta.kubernetes.io/os",
              "operator": "NotIn",
              "values": [
                "windows"
              ]
            }
          ]
        }
      ]
    },
    "preferredDuringSchedulingIgnoredDuringExecution": [
      {
        "weight": 100,
        "preference": {
          "matchExpressions": [
            {
              "key": "node-role.kubernetes.io/controlplane",
              "operator": "In",
              "values": [
                "true"
              ]
            }
          ]
        }
      },
      {
        "weight": 100,
        "preference": {
          "matchExpressions": [
            {
              "key": "node-role.kubernetes.io/control-plane",
              "operator": "In",
              "values": [
                "true"
              ]
            }
          ]
        }
      },
      {
        "weight": 100,
        "preference": {
          "matchExpressions": [
            {
              "key": "node-role.kubernetes.io/master",
              "operator": "In",
              "values": [
                "true"
              ]
            }
          ]
        }
      },
      {
        "weight": 1,
        "preference": {
          "matchExpressions": [
            {
              "key": "cattle.io/cluster-agent",
              "operator": "In",
              "values": [
                "true"
              ]
            }
          ]
        }
      }
    ]
  },
  "podAntiAffinity": {
    "preferredDuringSchedulingIgnoredDuringExecution": [
      {
        "weight": 100,
        "podAffinityTerm": {
          "labelSelector": {
            "matchExpressions": [
              {
                "key": "app",
                "operator": "In",
                "values": [
                  "cattle-cluster-agent"
                ]
              }
            ]
          },
          "topologyKey": "kubernetes.io/hostname"
        }
      }
    ]
  }
}`
	// FleetAgentAffinity is hardcoded in the rancher/fleet repo in the agent manifest
	// https://github.com/rancher/fleet/blob/90e33140906ba5d4931b4e1dee588854cbb300b1/pkg/agent/manifest.go#L178-L195
	FleetAgentAffinity = `{
  "nodeAffinity": {
    "preferredDuringSchedulingIgnoredDuringExecution": [
      {
        "weight": 1,
        "preference": {
          "matchExpressions": [
            {
              "key": "fleet.cattle.io/agent",
              "operator": "In",
              "values": [
                "true"
              ]
            }
          ]
        }
      }
    ]
  }
}`
)
