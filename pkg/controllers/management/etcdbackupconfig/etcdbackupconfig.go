package etcdbackupconfig

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CheckInterval = time.Minute * 1
)

type Controller struct {
	ctx                context.Context
	clusterClient      v3.ClusterInterface
	backupClient       v3.EtcdBackupInterface
	backupConfigClient v3.EtcdBackupConfigInterface
}

func Register(ctx context.Context, management *config.ManagementContext) {

	c := &Controller{
		ctx:                ctx,
		clusterClient:      management.Management.Clusters(""),
		backupClient:       management.Management.EtcdBackups(""),
		backupConfigClient: management.Management.EtcdBackupConfigs(""),
	}

	go c.syncBackups(ctx, CheckInterval)
}

func (c *Controller) syncBackups(ctx context.Context, i time.Duration) {
	// main backup config sync loop
	for range ticker.Context(ctx, i) {
		// get available backup configs
		backupConfigs, err := c.backupConfigClient.List(metav1.ListOptions{})
		if err != nil {
			logrus.Infof("melsayed failed to list configs: %v", err)
			continue
		}

		//Work on found configs
		for _, bc := range backupConfigs.Items {
			lastSeen := getCondition(&bc, "LastSeen")
			if lastSeen != nil {
				logrus.Infof("melsayed found backupConfig %s, lastSeen: %s", bc.Name, getCondition(&bc, "LastSeen").LastUpdateTime)
			}
			// set seen condition, probably don't need this anymore.
			if err := c.setSeenCondition(bc.Name); err != nil {
				logrus.Errorf("melsayed failed to setSeen: %v", err)
			}
			// set ownerReferences
			if len(bc.OwnerReferences) == 0 {
				controller := true
				cluster, _ := c.getCluster(&bc)
				r := []metav1.OwnerReference{
					{
						Name:       cluster.Name,
						UID:        cluster.UID,
						APIVersion: cluster.APIVersion,
						Kind:       cluster.Kind,
						Controller: &controller,
					},
				}
				c.updateBackupConfigOwnerReferences(bc.Name, r)
			}
			configDuration, err := time.ParseDuration(bc.Creation)
			if err != nil {
				logrus.Infof("melsayed can't parse duration %s : %v ", bc.Creation, err)
				continue
			}

			if err := c.setLastStartedCondition(bc.Name); err != nil {
				logrus.Errorf("melsayed failed to setLastStarted: %v", err)
				continue
			}
			lastCompleted := getCondition(&bc, "LastCompleted")
			if lastCompleted == nil {
				logrus.Infof("melsayed first time backup")
				c.kickBackup(bc.Name)
				continue
			}
			lastCompletedTime, _ := time.Parse(time.RFC3339, lastCompleted.LastUpdateTime)
			// It's been more than bc.Creation since we ran last successfull backup
			if time.Since(lastCompletedTime) > configDuration {
				c.kickBackup(bc.Name)
			}
		}
	}

}

func (c *Controller) updateBackupConfigOwnerReferences(backupConfigName string, r []metav1.OwnerReference) error {
	bc, _ := c.getBackupConfig(backupConfigName)
	bc.SetOwnerReferences(r)
	return c.updateBackupConfig(bc)
}

func (c *Controller) setSeenCondition(backupConfigName string) error {
	bc, _ := c.getBackupConfig(backupConfigName)
	setCondition(bc, v3.EtcdBackupConfigCondition{
		Type:           "LastSeen",
		Status:         "True",
		LastUpdateTime: time.Now().Format(time.RFC3339),
	})

	return c.updateBackupConfig(bc)
}

func (c *Controller) setLastStartedCondition(backupConfigName string) error {
	bc, err := c.getBackupConfig(backupConfigName)
	if err != nil {
		return err
	}
	setCondition(bc, v3.EtcdBackupConfigCondition{
		Type:           "LastStarted",
		Status:         "True",
		LastUpdateTime: time.Now().Format(time.RFC3339),
	})

	return c.updateBackupConfig(bc)
}

func (c *Controller) setLastCompletedConddition(backupConfigName string) error {
	bc, err := c.getBackupConfig(backupConfigName)
	if err != nil {
		return err
	}
	setCondition(bc, v3.EtcdBackupConfigCondition{
		Type:           "LastCompleted",
		Status:         "True",
		LastUpdateTime: time.Now().Format(time.RFC3339),
	})

	return c.updateBackupConfig(bc)
}

func (c *Controller) getBackupConfig(name string) (*v3.EtcdBackupConfig, error) {
	return c.backupConfigClient.Get(name, metav1.GetOptions{})
}

func (c *Controller) updateBackupConfig(bc *v3.EtcdBackupConfig) error {
	_, err := c.backupConfigClient.Update(bc)
	return err
}

func setCondition(backupConfig *v3.EtcdBackupConfig, newCondition v3.EtcdBackupConfigCondition) {
	conditions := []v3.EtcdBackupConfigCondition{}
	for _, c := range backupConfig.Status.Conditions {
		if c.Type == newCondition.Type {
			continue
		}
		conditions = append(conditions, c)
	}
	backupConfig.Status.Conditions = append(conditions, newCondition)
}

func getCondition(backupConfig *v3.EtcdBackupConfig, cType string) *v3.EtcdBackupConfigCondition {
	for _, c := range backupConfig.Status.Conditions {
		if c.Type == cType {
			return &c
		}
	}
	return nil
}

func (c *Controller) getCluster(backupConfig *v3.EtcdBackupConfig) (*v3.Cluster, error) {
	return c.clusterClient.Get(backupConfig.ClusterName, metav1.GetOptions{})
}

func (c *Controller) kickBackup(backupConfigName string) {
	logrus.Infof("melsayed doing the backup")
	controller := true
	backupConfig, _ := c.getBackupConfig(backupConfigName)
	cluster, _ := c.getCluster(backupConfig)
	newBackup := &v3.EtcdBackup{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", backupConfigName),
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       backupConfigName,
					UID:        backupConfig.UID,
					APIVersion: backupConfig.APIVersion,
					Kind:       backupConfig.Kind,
				},
				{
					Name:       cluster.Name,
					UID:        cluster.UID,
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Controller: &controller,
				},
			},
		},
		BackupConfig: backupConfigName,
		Status: v3.EtcdBackupStatus{
			Conditions: []v3.EtcdBackupCondition{
				v3.EtcdBackupCondition{
					Type:           "Created",
					Status:         v1.ConditionTrue,
					LastUpdateTime: time.Now().Format(time.RFC3339),
				},
			},
		},
	}
	_, err := c.backupClient.Create(newBackup)
	if err != nil {
		logrus.Errorf("melsayed failed to cteate backup: %v", err)
		return
	}
	logrus.Infof("melsayed done with the backup")
}
