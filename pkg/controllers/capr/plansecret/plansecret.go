package plansecret

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	sb "github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v2/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

type handler struct {
	secrets             corecontrollers.SecretClient
	machinesCache       capicontrollers.MachineCache
	machinesClient      capicontrollers.MachineClient
	etcdSnapshotsClient rkev1controllers.ETCDSnapshotClient
	etcdSnapshotsCache  rkev1controllers.ETCDSnapshotCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		secrets:             clients.Core.Secret(),
		machinesCache:       clients.CAPI.Machine().Cache(),
		machinesClient:      clients.CAPI.Machine(),
		etcdSnapshotsClient: clients.RKE.ETCDSnapshot(),
		etcdSnapshotsCache:  clients.RKE.ETCDSnapshot().Cache(),
	}
	clients.Core.Secret().OnChange(ctx, "plan-secret", h.OnChange)
}

func (h *handler) OnChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Type != capr.SecretTypeMachinePlan || len(secret.Data) == 0 {
		return secret, nil
	}

	logrus.Debugf("[plansecret] reconciling secret %s/%s", secret.Namespace, secret.Name)

	node, err := planner.SecretToNode(secret)
	if err != nil {
		return secret, err
	}

	if v, ok := node.PeriodicOutput["etcd-snapshot-list-local"]; ok && v.ExitCode == 0 && len(v.Stdout) > 0 {
		if err := h.reconcileEtcdSnapshotList(secret, false, v.Stdout); err != nil {
			logrus.Errorf("[plansecret] error reconciling local snapshot list for secret %s/%s: %v", secret.Namespace, secret.Name, err)
		}
	}

	if v, ok := node.PeriodicOutput["etcd-snapshot-list-s3"]; ok && v.ExitCode == 0 && len(v.Stdout) > 0 && secret.Labels[capr.InitNodeLabel] == "true" {
		if err := h.reconcileEtcdSnapshotList(secret, true, v.Stdout); err != nil {
			logrus.Errorf("[plansecret] error reconciling S3 snapshot list for secret %s/%s: %v", secret.Namespace, secret.Name, err)
		}
	}

	appliedChecksum := string(secret.Data["applied-checksum"])
	failedChecksum := string(secret.Data["failed-checksum"])
	plan := secret.Data["plan"]

	secretChanged := false
	secret = secret.DeepCopy()

	if appliedChecksum == planner.PlanHash(plan) && !bytes.Equal(plan, secret.Data["appliedPlan"]) {
		secret.Data["appliedPlan"] = plan
		secretChanged = true
	}

	if len(secret.Data["probe-statuses"]) > 0 {
		_, healthy, err := planner.ParseProbeStatuses(secret.Data["probe-statuses"])
		if err != nil {
			return nil, err
		}
		if healthy && secret.Annotations[capr.PlanProbesPassedAnnotation] == "" {
			// a non-zero value for this annotation indicates the probes for this specific plan have passed at least once
			secret.Annotations[capr.PlanProbesPassedAnnotation] = time.Now().UTC().Format(time.RFC3339)
			secretChanged = true
		}
	}

	if secretChanged {
		// don't return the secret at this point, we want to attempt to update the machine status later on
		secret, err = h.secrets.Update(secret)
		if err != nil {
			return secret, err
		}
	}

	if failedChecksum == planner.PlanHash(plan) {
		logrus.Debugf("[plansecret] %s/%s: rv: %s: Detected failed plan application, reconciling machine PlanApplied condition to error", secret.Namespace, secret.Name, secret.ResourceVersion)
		err = h.reconcileMachinePlanAppliedCondition(secret, fmt.Errorf("error applying plan -- check rancher-system-agent.service logs on node for more information"))
		return secret, err
	}

	logrus.Debugf("[plansecret] %s/%s: rv: %s: Reconciling machine PlanApplied condition to nil", secret.Namespace, secret.Name, secret.ResourceVersion)
	err = h.reconcileMachinePlanAppliedCondition(secret, nil)
	return secret, err
}

func (h *handler) reconcileMachinePlanAppliedCondition(secret *corev1.Secret, planAppliedErr error) error {
	if secret == nil {
		logrus.Debug("[plansecret] secret was nil when reconciling machine status")
		return nil
	}

	condition := capi.ConditionType(capr.PlanApplied)

	machineName, ok := secret.Labels[capr.MachineNameLabel]
	if !ok {
		return fmt.Errorf("did not find machine label on secret %s/%s", secret.Namespace, secret.Name)
	}

	machine, err := h.machinesCache.Get(secret.Namespace, machineName)
	if err != nil {
		return err
	}

	machine = machine.DeepCopy()

	var needsUpdate bool
	if planAppliedErr != nil &&
		(conditions.GetMessage(machine, condition) != planAppliedErr.Error() ||
			*conditions.GetSeverity(machine, condition) != capi.ConditionSeverityError ||
			!conditions.IsFalse(machine, condition) ||
			conditions.GetReason(machine, condition) != "Error") {
		logrus.Debugf("[plansecret] machine %s/%s: marking PlanApplied as false", machine.Namespace, machine.Name)
		conditions.MarkFalse(machine, condition, "Error", capi.ConditionSeverityError, planAppliedErr.Error())
		needsUpdate = true
	} else if planAppliedErr == nil && !conditions.IsTrue(machine, condition) {
		logrus.Debugf("[plansecret] machine %s/%s: marking PlanApplied as true", machine.Namespace, machine.Name)
		conditions.MarkTrue(machine, condition)
		needsUpdate = true
	}

	if needsUpdate {
		logrus.Debugf("[plansecret] machine %s/%s: updating status of machine to reconcile for condition with error: %+v", machine.Namespace, machine.Name, planAppliedErr)
		_, err = h.machinesClient.UpdateStatus(machine)
	}

	return err
}

func machineOwnerRef(machine capi.Machine) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         machine.APIVersion,
		Kind:               machine.Kind,
		Name:               machine.Name,
		UID:                machine.UID,
		Controller:         &[]bool{true}[0],
		BlockOwnerDeletion: &[]bool{true}[0],
	}
}

func (h *handler) reconcileEtcdSnapshotList(secret *corev1.Secret, s3 bool, listStdout []byte) error {
	cnl := secret.Labels[capr.ClusterNameLabel]
	if len(cnl) == 0 {
		return fmt.Errorf("node secret did not have label %s", capr.ClusterNameLabel)
	}

	machineName, ok := secret.Labels[capr.MachineNameLabel]
	if !ok {
		return fmt.Errorf("did not find machine label on secret %s/%s", secret.Namespace, secret.Name)
	}

	ls := labels.SelectorFromSet(map[string]string{
		capr.ClusterNameLabel: cnl,
		capr.NodeNameLabel:    "s3",
	})

	var machine *capi.Machine
	var machineID string
	var err error

	if !s3 {
		machine, err = h.machinesCache.Get(secret.Namespace, machineName)
		if err != nil {
			return err
		}
		if machine.Labels[capr.MachineIDLabel] != "" {
			machineID = machine.Labels[capr.MachineIDLabel]
			ls = labels.SelectorFromSet(map[string]string{
				capr.ClusterNameLabel: cnl,
				capr.MachineIDLabel:   machine.Labels[capr.MachineIDLabel],
			})
		} else {
			return fmt.Errorf("error finding machine ID for machine %s/%s", machine.Namespace, machine.Name)
		}
	}

	etcdSnapshotsOnNode := outputToEtcdSnapshots(cnl, listStdout)

	etcdSnapshots, err := h.etcdSnapshotsCache.List(secret.Namespace, ls)
	if err != nil {
		return err
	}

	// indexedEtcdSnapshots is a map of etcd snapshots that already exist in the management cluster
	indexedEtcdSnapshots := map[string]*v1.ETCDSnapshot{}

	for _, v := range etcdSnapshots {
		if _, ok := etcdSnapshotsOnNode[v.Name]; !ok && v.Status.Missing {
			// delete the etcd snapshot as it is missing
			logrus.Infof("[plansecret] Deleting etcd snapshot %s/%s", v.Namespace, v.Name)
			if err := h.etcdSnapshotsClient.Delete(v.Namespace, v.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
		indexedEtcdSnapshots[v.Name] = v
	}

	if !s3 && machine.Status.NodeRef != nil {
		for k, v := range etcdSnapshotsOnNode {
			if _, ok := indexedEtcdSnapshots[k]; !ok {
				// create the etcdsnapshot object as it was not in the list of etcdsnapshots and not an S3 snapshot
				snapshot := v1.ETCDSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: secret.Namespace,
						Labels: map[string]string{
							capr.ClusterNameLabel: cnl,
							capr.MachineIDLabel:   machineID,
						},
						Annotations: map[string]string{
							sb.SnapshotNameKey:      v.Name,
							sb.StorageAnnotationKey: sb.StorageLocal,
						},
						OwnerReferences: []metav1.OwnerReference{
							machineOwnerRef(*machine),
						},
					},
					Spec: v1.ETCDSnapshotSpec{
						ClusterName: cnl,
					},
					SnapshotFile: v1.ETCDSnapshotFile{
						Name:     v.Name,
						Location: v.Location,
						NodeName: machine.Status.NodeRef.Name,
					},
				}
				snapshot.Name = name.SafeConcatName(cnl, snapshot.SnapshotFile.Name, sb.StorageLocal)
				logrus.Debugf("[plansecret] machine %s/%s: creating etcd snapshot %s for cluster %s", machine.Namespace, machine.Name, snapshot.Name, cnl)
				_, err = h.etcdSnapshotsClient.Create(&snapshot)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return fmt.Errorf("error while creating etcd snapshot: %w", err)
				}
			}
		}
	}

	return nil
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
			Name:    strings.ToLower(sb.InvalidKeyChars.ReplaceAllString(snapshotData[0], "-")),
			Size:    snapshotData[1],
			Created: snapshotData[2],
			S3:      true,
		}, nil
	case 4:
		return &snapshot{
			Name:     strings.ToLower(sb.InvalidKeyChars.ReplaceAllString(snapshotData[0], "-")),
			Location: snapshotData[1],
			Size:     snapshotData[2],
			Created:  snapshotData[3],
			S3:       false,
		}, nil
	}
	return nil, fmt.Errorf("input (%s) did not have 3 or 4 fields", input)
}
