package multiclusterapp

import (
	"context"
	"fmt"
	"github.com/rancher/norman/condition"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

//StartMCAppStateController gets all corresponding apps and update condition on multi cluster app sync
func StartMCAppStateController(ctx context.Context, cluster *config.UserContext) {
	mcApps := cluster.Management.Management.MultiClusterApps("")
	s := &MCAppStateController{
		Apps:             cluster.Management.Project.Apps("").Controller().Lister(),
		MultiClusterApps: mcApps,
	}
	mcApps.AddHandler(ctx, "multi-cluster-app-state-controller", s.sync)
}

type MCAppStateController struct {
	Apps             pv3.AppLister
	MultiClusterApps v3.MultiClusterAppInterface
}

func (m *MCAppStateController) sync(key string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	if mcapp == nil || mcapp.DeletionTimestamp != nil {
		return mcapp, nil
	}
	var apps []*pv3.App
	for _, t := range mcapp.Spec.Targets {
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return mcapp, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
		}
		projectNS := split[1]
		if t.AppName != "" {
			app, err := m.Apps.Get(projectNS, t.AppName)
			if err != nil {
				return mcapp, err
			}
			if value, ok := app.Labels[multiClusterAppIDSelector]; !ok || value != mcapp.Name {
				return mcapp, fmt.Errorf("app %s missing label selector for %s", t.AppName, mcapp.Name)
			}
			apps = append(apps, app)
		}
	}
	toUpdate := mcapp.DeepCopy()
	if len(apps) != len(mcapp.Spec.Targets) {
		v3.MultiClusterAppConditionInstalled.Unknown(toUpdate)
		v3.MultiClusterAppConditionInstalled.Message(toUpdate, "creating apps")
		return m.MultiClusterApps.Update(toUpdate)
	}
	updating := map[string]bool{}
	installing := map[string]bool{}
	for _, app := range apps {
		if !pv3.AppConditionInstalled.IsTrue(app) {
			installing[app.Name] = true
		} else if !pv3.AppConditionDeployed.IsTrue(app) && pv3.AppConditionDeployed.GetStatus(app) != "" {
			updating[app.Name] = true
		}
	}
	if checkForUpdate(installing, toUpdate, v3.MultiClusterAppConditionInstalled, "creating") {
		return m.MultiClusterApps.Update(toUpdate)
	}

	if checkForUpdate(updating, toUpdate, v3.MultiClusterAppConditionDeployed, "updating") {
		return m.MultiClusterApps.Update(toUpdate)
	}

	return mcapp, nil
}

func checkForUpdate(transitioning map[string]bool, mcapp *v3.MultiClusterApp, cond condition.Cond, reason string) bool {
	if len(transitioning) > 0 {
		existing := strings.Split(cond.GetMessage(mcapp), ",")
		if !Equal(existing, transitioning) {
			cond.Unknown(mcapp)
			cond.Reason(mcapp, fmt.Sprintf("%s apps", reason))
			cond.Message(mcapp, GetMsg(transitioning))
			return true
		}
	} else if !cond.IsTrue(mcapp) {
		cond.True(mcapp)
		cond.Reason(mcapp, "")
		cond.Message(mcapp, "")
		return true
	}
	return false
}

func GetMsg(data map[string]bool) string {
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	return fmt.Sprintf("%s %s", "Updating", strings.Join(keys, ","))
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
