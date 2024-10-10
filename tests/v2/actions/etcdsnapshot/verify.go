package etcdsnapshot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	extdefault "github.com/rancher/shepherd/extensions/defaults"
	shepherdsnapshot "github.com/rancher/shepherd/extensions/etcdsnapshot"
	"github.com/sirupsen/logrus"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// VerifySnapshots verifies that all snapshots come to an active state, and the correct number
// of snapshots were taken based on the number of nodes and snapshot type (s3 vs. local)
func VerifySnapshots(client *rancher.Client, clusterName string, snapshotIDs []string, isRKE1 bool) error {
	isS3 := false
	err := kwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, extdefault.FiveMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		var snapshotObj any
		var state string

		for _, snapshotID := range snapshotIDs {
			if isRKE1 {
				snapshotObj, err = client.Management.EtcdBackup.ByID(snapshotID)
				state = snapshotObj.(*management.EtcdBackup).State
			} else {
				snapshotObj, err = client.Steve.SteveType(shepherdsnapshot.SnapshotSteveResourceType).ByID(snapshotID)
				state = snapshotObj.(*steveV1.SteveAPIObject).ObjectMeta.State.Name

				if strings.HasSuffix(snapshotObj.(*steveV1.SteveAPIObject).Name, "-s3") {
					isS3 = true
				}
			}

			if err != nil {
				return false, nil
			}

			if state != "active" {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	expectedNewSnapshots, _ := MatchNodeToAnyEtcdRole(client, clusterID)

	if isRKE1 {
		expectedNewSnapshots = 1
	} else if isS3 {
		expectedNewSnapshots = expectedNewSnapshots * 2
	}

	if expectedNewSnapshots != len(snapshotIDs) {
		logrus.Info(snapshotIDs)
		return fmt.Errorf("unexpected number of snapshots. Expected %v but have %v", expectedNewSnapshots, len(snapshotIDs))
	}

	return nil
}
