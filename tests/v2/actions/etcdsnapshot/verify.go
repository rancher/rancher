package etcdsnapshot

import (
	"context"
	"fmt"
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

const (
	expectedRKE1SnapshotLen = 1
)

// VerifyRKE1Snapshots verifies that all snapshots come to an active state, and the correct number
// of snapshots were taken based on the number of nodes and snapshot type (s3 vs. local)
func VerifyRKE1Snapshots(client *rancher.Client, clusterName string, snapshotIDs []string) error {
	err := kwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, extdefault.FiveMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		var snapshotObj any
		var state string

		for _, snapshotID := range snapshotIDs {
			snapshotObj, err = client.Management.EtcdBackup.ByID(snapshotID)
			state = snapshotObj.(*management.EtcdBackup).State

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

	// RKE1 doesn't give us snapshot-per-node. It gives us one object representing the snapshot for the cluster.
	if expectedRKE1SnapshotLen != len(snapshotIDs) {
		logrus.Info(snapshotIDs)
		return fmt.Errorf("unexpected number of snapshots. Expected %v but have %v", expectedRKE1SnapshotLen, len(snapshotIDs))
	}

	return nil
}

// VerifyV2ProvSnapshots verifies that all snapshots come to an active state, and the correct number
// of snapshots were taken based on the number of nodes and snapshot type (s3 vs. local)
func VerifyV2ProvSnapshots(client *rancher.Client, clusterName string, snapshotIDs []string) error {
	isS3 := false
	err := kwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, extdefault.FiveMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		var snapshotObj any
		var state string

		for _, snapshotID := range snapshotIDs {
			snapshotObj, err = client.Steve.SteveType(shepherdsnapshot.SnapshotSteveResourceType).ByID(snapshotID)
			state = snapshotObj.(*steveV1.SteveAPIObject).ObjectMeta.State.Name

			store, ok := snapshotObj.(*steveV1.SteveAPIObject).Annotations["etcdsnapshot.rke.io/storage"]
			if ok && store == "s3" {
				isS3 = true
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

	if isS3 {
		expectedNewSnapshots = expectedNewSnapshots * 2
	}

	if expectedNewSnapshots != len(snapshotIDs) {
		logrus.Info(snapshotIDs)
		return fmt.Errorf("unexpected number of snapshots. Expected %v but have %v", expectedNewSnapshots, len(snapshotIDs))
	}

	return nil
}
