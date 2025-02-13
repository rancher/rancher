package clusters

import (
	"errors"
	"net/url"

	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/sirupsen/logrus"
)

const (
	LabelWorker = "labelSelector=node-role.kubernetes.io/worker=true"
)

var (
	SmallerPoolClusterSize = errors.New("Machine pool cluster size is smaller than expected pool size")
)

// VerifyNodePoolSize is a helper function that checks if the machine pool cluster size is greater than or equal to poolSize
func VerifyNodePoolSize(steveClient *steveV1.Client, labelSelector string, poolSize int) error {
	logrus.Info("Checking node pool")

	logrus.Infof("Getting the node using the label [%v]", labelSelector)
	query, err := url.ParseQuery(labelSelector)
	if err != nil {
		return err
	}

	nodeList, err := steveClient.SteveType("node").List(query)
	if err != nil {
		return err
	}

	if len(nodeList.Data) < poolSize {
		return SmallerPoolClusterSize
	}

	return nil
}
