package helm

import (
	"context"
	"fmt"
	"strings"

	v33 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/rancher/norman/types/convert"
	util "github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	projectLabel = "field.cattle.io/projectId"
)

func StartStateCalculator(ctx context.Context, cluster *config.UserContext) {
	apps := cluster.Management.Project.Apps("")
	w := &AppStateCalculator{
		appLister:        apps.Controller().Lister(),
		apps:             apps,
		multiClusterApps: cluster.Management.Management.MultiClusterApps(""),
		namespaceLister:  cluster.Core.Namespaces("").Controller().Lister(),
		projectLister:    cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		clusterName:      cluster.ClusterName,
	}
	w.workloadLister = util.NewWorkloadController(ctx, cluster.UserOnlyContext(), w.sync)
	apps.Controller().AddHandler(ctx, "app-state-controller", w.syncAppState)
}

type AppStateCalculator struct {
	apps             pv3.AppInterface
	appLister        pv3.AppLister
	namespaceLister  corev1.NamespaceLister
	multiClusterApps v3.MultiClusterAppInterface
	workloadLister   util.CommonController
	projectLister    v3.ProjectLister
	clusterName      string
}

func (s *AppStateCalculator) sync(key string, obj *util.Workload) error {
	if obj == nil {
		return nil
	}
	if label, ok := obj.Labels[appLabel]; ok {
		ns, err := s.namespaceLister.Get("", obj.Namespace)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
			logrus.Errorf("error getting namespace %s for app %s: %v", obj.Namespace, obj.Name, err)
			return nil
		}
		projectNS := ns.Labels[projectLabel]
		if projectNS == "" {
			return fmt.Errorf("cannot find app namespace in labels of %s", obj.Namespace)
		}
		app, err := s.appLister.Get(projectNS, label)
		if err != nil {
			if errors.IsNotFound(err) {
				// If Rancher is unaware of an app, we should ignore tracking its state
				// A non-existent app is likely managed by another Rancher (e.g. Hosted Rancher)
				return nil
			}
			if !errors.IsNotFound(err) {
				return err
			}
			logrus.Errorf("error getting app %s %v", label, err)
			return nil
		}
		if app != nil && app.DeletionTimestamp == nil {
			s.apps.Controller().Enqueue(app.Namespace, app.Name)
		}
	}
	return nil
}

func (s *AppStateCalculator) syncAppState(key string, app *pv3.App) (runtime.Object, error) {
	if app == nil || app.DeletionTimestamp != nil {
		return nil, nil
	}
	if !v32.AppConditionInstalled.IsTrue(app) {
		return app, nil
	}
	project, err := s.projectLister.Get(s.clusterName, app.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		return app, err
	}
	if err != nil || project == nil || project.DeletionTimestamp != nil {
		return app, nil
	}
	workloads, err := s.getWorkloadsByApp(app.Name)
	if err != nil {
		return app, fmt.Errorf("error getting workloads %v", err)
	}
	updatingWorkloads := getUpdating(workloads)
	toUpdate := app.DeepCopy()
	if len(updatingWorkloads) == 0 {
		if v32.AppConditionDeployed.IsTrue(toUpdate) {
			return app, nil
		}
		v32.AppConditionDeployed.True(toUpdate)
		v32.AppConditionDeployed.Reason(toUpdate, "")
		v32.AppConditionDeployed.Message(toUpdate, "")
	} else {
		existing := strings.Split(v33.MultiClusterAppConditionDeployed.GetMessage(toUpdate), ",")
		if Equal(existing, updatingWorkloads) {
			return app, nil
		}
		v32.AppConditionDeployed.Unknown(toUpdate)
		v32.AppConditionDeployed.Message(toUpdate, GetMsg(updatingWorkloads))
		v32.AppConditionDeployed.Reason(toUpdate, "workloads are updating")
	}
	return s.apps.Update(toUpdate)
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

func GetMsg(data map[string]bool) string {
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	return fmt.Sprintf("%s", strings.Join(keys, ","))
}

func Equal(existing []string, toUpdate map[string]bool) bool {
	if len(existing) != len(toUpdate) {
		return false
	}
	for _, name := range existing {
		if _, ok := toUpdate[name]; !ok {
			return false
		}
	}
	return true
}
