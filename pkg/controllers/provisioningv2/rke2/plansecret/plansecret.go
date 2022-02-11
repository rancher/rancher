package plansecret

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type handler struct {
	secrets             corecontrollers.SecretClient
	machinesCache       capicontrollers.MachineCache
	etcdSnapshotsClient rkev1controllers.ETCDSnapshotClient
	etcdSnapshotsCache  rkev1controllers.ETCDSnapshotCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		secrets:             clients.Core.Secret(),
		machinesCache:       clients.CAPI.Machine().Cache(),
		etcdSnapshotsClient: clients.RKE.ETCDSnapshot(),
		etcdSnapshotsCache:  clients.RKE.ETCDSnapshot().Cache(),
	}
	clients.Core.Secret().OnChange(ctx, "plan-secret", h.OnChange)
}

func (h *handler) OnChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Type != rke2.SecretTypeMachinePlan || len(secret.Data) == 0 {
		return secret, nil
	}

	node, err := planner.SecretToNode(secret)
	if err != nil {
		return secret, err
	}

	if v, ok := node.PeriodicOutput["etcd-snapshot-list"]; ok && v.ExitCode == 0 && len(v.Stdout) > 0 {
		cnl := secret.Labels[rke2.ClusterNameLabel]
		if len(cnl) == 0 {
			return secret, fmt.Errorf("node secret did not have label %s", rke2.ClusterNameLabel)
		}

		machineName, ok := secret.Labels[rke2.MachineNameLabel]
		if !ok {
			return secret, fmt.Errorf("did not find machine label on secret %s/%s", secret.Namespace, secret.Name)
		}

		machine, err := h.machinesCache.Get(secret.Namespace, machineName)
		if err != nil {
			return secret, err
		}

		if machine.Status.NodeRef != nil && machine.Status.NodeRef.Name != "" {
			etcdSnapshotsOnNode := outputToEtcdSnapshots(cnl, v.Stdout)
			ls, err := labels.Parse(fmt.Sprintf("%s=%s,%s=%s", rke2.ClusterNameLabel, cnl, rke2.NodeNameLabel, machine.Status.NodeRef.Name))
			if err != nil {
				return secret, err
			}

			etcdSnapshots, err := h.etcdSnapshotsCache.List(secret.Namespace, ls)
			if err != nil {
				return secret, err
			}

			for _, v := range etcdSnapshots {
				if _, ok := etcdSnapshotsOnNode[v.Name]; !ok && v.Status.Missing {
					// delete the etcd snapshot as it is likely missing
					logrus.Infof("Deleting etcd snapshot %s/%s", v.Namespace, v.Name)
					if err := h.etcdSnapshotsClient.Delete(v.Namespace, v.Name, &metav1.DeleteOptions{}); err != nil {
						return secret, err
					}
				}
			}
		}
	}

	appliedChecksum := string(secret.Data["applied-checksum"])
	plan := secret.Data["plan"]
	appliedPlan := secret.Data["appliedPlan"]

	if appliedChecksum == hash(plan) && !bytes.Equal(plan, appliedPlan) {
		secret = secret.DeepCopy()
		secret.Data["appliedPlan"] = plan
		return h.secrets.Update(secret)
	}

	return secret, nil
}

type snapshot struct {
	Name     string
	Location string
	Size     string
	Created  string
	S3       bool
}

func outputToEtcdSnapshots(clusterName string, collectedOutput []byte) map[string]*snapshot {
	scanner := bufio.NewScanner(bytes.NewBuffer(collectedOutput))
	snapshots := make(map[string]*snapshot)
	for scanner.Scan() {
		line := scanner.Text()
		if s := strings.Fields(line); len(s) == 3 || len(s) == 4 {
			switch len(s) {
			case 3:
				if strings.ToLower(s[0]) == "name" &&
					strings.ToLower(s[1]) == "size" &&
					strings.ToLower(s[2]) == "created" {
					continue
				}
			case 4:
				if strings.ToLower(s[0]) == "name" &&
					strings.ToLower(s[1]) == "location" &&
					strings.ToLower(s[2]) == "size" &&
					strings.ToLower(s[3]) == "created" {
					continue
				}
			}
			ss, err := generateEtcdSnapshotFromListOutput(line)
			if err != nil {
				logrus.Errorf("error parsing etcd snapshot output (%s) to etcd snapshot: %v", line, err)
				continue
			}
			suffix := "local"
			if ss.S3 {
				suffix = "s3"
			}
			snapshots[fmt.Sprintf("%s-%s-%s", clusterName, ss.Name, suffix)] = ss
		}
	}
	return snapshots
}

func generateEtcdSnapshotFromListOutput(input string) (*snapshot, error) {
	snapshotData := strings.Fields(input)
	switch len(snapshotData) {
	case 3:
		return &snapshot{
			Name:    snapshotData[0],
			Size:    snapshotData[1],
			Created: snapshotData[2],
			S3:      true,
		}, nil
	case 4:
		return &snapshot{
			Name:     snapshotData[0],
			Location: snapshotData[1],
			Size:     snapshotData[2],
			Created:  snapshotData[3],
			S3:       false,
		}, nil
	}
	return nil, fmt.Errorf("input (%s) did not have 3 or 4 fields", input)
}

func hash(plan []byte) string {
	result := sha256.Sum256(plan)
	return hex.EncodeToString(result[:])
}
