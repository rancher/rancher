package helm

import (
	"context"
	"fmt"
	"github.com/rancher/norman/types/convert"
	mcapp "github.com/rancher/rancher/pkg/controllers/user/multiclusterapp"
	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/namespace"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

const (
	MultiClusterAppIDSelector = "mcapp"
	projectLabel              = "field.cattle.io/projectId"
)

func StartStateCalculator(ctx context.Context, cluster *config.UserContext) {
	apps := cluster.Management.Project.Apps("")
	w := &AppStateCalculator{
		AppLister:        apps.Controller().Lister(),
		Apps:             apps,
		MultiClusterApps: cluster.Management.Management.MultiClusterApps(""),
		NamespaceLister:  cluster.Core.Namespaces("").Controller().Lister(),
	}
	w.workloadLister = util.NewWorkloadController(ctx, cluster.UserOnlyContext(), w.sync)
	apps.Controller().AddHandler(ctx, "app-state-controller", w.syncAppState)
}

type AppStateCalculator struct {
	Apps             pv3.AppInterface
	AppLister        pv3.AppLister
	NamespaceLister  corev1.NamespaceLister
	MultiClusterApps v3.MultiClusterAppInterface
	workloadLister   util.CommonController
}

func (s *AppStateCalculator) sync(key string, obj *util.Workload) error {
	if obj == nil {
		return nil
	}
	if label, ok := obj.Labels[appLabel]; ok {
		ns, err := s.NamespaceLister.Get("", obj.Namespace)
		if err != nil {
			return err
		}
		projectNS := ns.Labels[projectLabel]
		if projectNS == "" {
			return fmt.Errorf("cannot find app namespace in labels of %s", obj.Namespace)
		}
		app, err := s.AppLister.Get(projectNS, label)
		if err != nil {
			return fmt.Errorf("error getting app %s %v", label, err)
		}
		if app != nil && app.DeletionTimestamp == nil {
			s.Apps.Controller().Enqueue(app.Namespace, app.Name)
		}
	}
	return nil
}

func (s *AppStateCalculator) syncAppState(key string, app *pv3.App) (runtime.Object, error) {
	if app == nil || app.DeletionTimestamp != nil {
		return nil, nil
	}
	if !pv3.AppConditionInstalled.IsTrue(app) {
		return app, nil
	}
	workloads, err := s.getWorkloadsByApp(app.Name)
	if err != nil {
		return app, fmt.Errorf("error getting workloads %v", err)
	}
	updatingWorkloads := getUpdating(workloads)
	toUpdate := app.DeepCopy()
	if len(updatingWorkloads) == 0 {
		if pv3.AppConditionDeployed.IsTrue(toUpdate) {
			return app, nil
		}
		pv3.AppConditionDeployed.True(toUpdate)
		pv3.AppConditionDeployed.Reason(toUpdate, "")
		pv3.AppConditionDeployed.Message(toUpdate, "")
	} else {
		existing := strings.Split(strings.TrimPrefix(
			pv3.AppConditionDeployed.GetMessage(toUpdate), "Updating "), ",")
		if mcapp.Equal(existing, updatingWorkloads) {
			return app, nil
		}
		pv3.AppConditionDeployed.Unknown(toUpdate)
		pv3.AppConditionDeployed.Message(toUpdate, mcapp.GetMsg(updatingWorkloads))
		pv3.AppConditionDeployed.Reason(toUpdate, "workloads are updating")
	}
	updated, err := s.Apps.Update(toUpdate)
	if err != nil {
		logrus.Errorf("error updating app %s", toUpdate.Name)
	}
	if mcappName, ok := app.Labels[MultiClusterAppIDSelector]; ok {
		s.MultiClusterApps.Controller().Enqueue(namespace.GlobalNamespace, mcappName)
	}
	return updated, nil
}

func (s *AppStateCalculator) getWorkloadsByApp(name string) ([]*util.Workload, error) {
	label := map[string]string{
		appLabel: name,
	}
	workloads, err := s.workloadLister.GetWorkloadsMatchingSelector("", label)
	if err != nil {
		return nil, err
	}
	return workloads, nil
}

func getUpdating(workloads []*util.Workload) map[string]bool {
	updatingWorkloads := map[string]bool{}
	for _, workload := range workloads {
		if workload.Kind == util.ReplicaSetType {
			continue
		}
		if hasUpdatingCondition(workload.Status.Conditions) {
			updatingWorkloads[workload.Name] = true
		} else if workload.Status.Replicas != workload.Status.AvailableReplicas {
			updatingWorkloads[workload.Name] = true
		}
	}
	return updatingWorkloads
}

func hasUpdatingCondition(conditions []map[string]interface{}) bool {
	for _, condition := range conditions {
		status := convert.ToString(condition["status"])
		if status == "False" || status == "Unknown" {
			return true
		}
	}
	return false
}
