package functions

import "time"

func WaitForActiveCLuster(hostURL string, clusterName string, token string) {
	time.Sleep(10 * time.Second)

	id := GetClusterID(hostURL, clusterName, token)
	state := GetClusterState(hostURL, id, token)
	updating := false

	for state != "active" {
		for state != "active" && !updating {
			state = GetClusterState(hostURL, id, token)
			time.Sleep(10 * time.Second)
			if state == "updating" {
				updating = true
			}
		}
		state = GetClusterState(hostURL, id, token)
		time.Sleep(10 * time.Second)
	}
	time.Sleep(10 * time.Second)
}
