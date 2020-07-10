package cron

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	cronSchedule = "0 0 * * *" // every 24 hours
	jailPath     = "/opt/jail"
)

type jailSync struct {
	clusters v3.ClusterInterface
}

// StartJailSyncCron for cleaning up the /opt/jail dir
func StartJailSyncCron(scaledContext *config.ScaledContext) error {
	ref := &jailSync{
		clusters: scaledContext.Management.Clusters(""),
	}

	schedule, err := cron.ParseStandard(cronSchedule)
	if err != nil {
		return fmt.Errorf("error parsing auth refresh cron: %v", err)
	}

	c := cron.New()
	job := cron.FuncJob(ref.syncJails)
	c.Schedule(schedule, job)
	c.Start()

	return nil
}

// syncJails removes any unneeded jails from old clusters.
func (j *jailSync) syncJails() {
	// Get the clusters from the api to ensure we are up to date
	clusters, err := j.clusters.List(metav1.ListOptions{})
	if err != nil {
		logrus.Warnf("Error listing clusters for jail cleanup: %v", err)
	}

	clusterMap := make(map[string]v3.Cluster)
	for _, cluster := range clusters.Items {
		clusterMap[cluster.Name] = cluster
	}

	files, err := ioutil.ReadDir(jailPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Warnf("Error attempting to get files for jail cleanup: %v", err)
		}
		// The dir doesn't exist, nothing to do
		return
	}

	for _, file := range files {
		if file.IsDir() {
			dirName := file.Name()
			// Don't drop the KE driver jail
			if dirName == "driver-jail" {
				continue
			}

			// If the dir doesn't have a corresponding cluster delete it
			if _, ok := clusterMap[dirName]; !ok {
				clusterPath := path.Join(jailPath, dirName)
				err = os.RemoveAll(clusterPath)
				if err != nil {
					logrus.Warnf("Error attempting to delete jail %v: %v", clusterPath, err)
				}
			}
		}
	}
}
