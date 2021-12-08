package multiclusterapp

import (
	"context"
	"fmt"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	installing = "installing"
	deploying  = "deploying"
	active     = "active"
)

//StartMCAppStateController gets all corresponding apps and update condition on multi cluster app sync
func StartMCAppStateController(ctx context.Context, management *config.ManagementContext) {
	mcApps := management.Management.MultiClusterApps("")
	s := &MCAppStateController{
		Apps:             management.Project.Apps("").Controller().Lister(),
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
	mcappState := active
	if v32.MultiClusterAppConditionInstalled.IsUnknown(mcapp) && v32.MultiClusterAppConditionInstalled.GetMessage(mcapp) == "upgrading" {
		mcappState = ""
	}
	var toUpdate *v3.MultiClusterApp
	for ind, t := range mcapp.Spec.Targets {
		if t.AppName == "" {
			mcappState = installing
			continue
		}
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return mcapp, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
		}
		projectNS := split[1]
		app, err := m.Apps.Get(projectNS, t.AppName)
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Debugf("app %s not found for mcapp %s in projectNS %s", t.AppName, mcapp.Name, projectNS)
				if mcappState != "" {
					mcappState = installing
				}
				continue
			}
			return mcapp, err
		}
		if value, ok := app.Labels[MultiClusterAppIDSelector]; !ok || value != mcapp.Name {
			return mcapp, fmt.Errorf("app %s missing label selector for %s", t.AppName, mcapp.Name)
		}
		if !v33.AppConditionInstalled.IsTrue(app) {
			toUpdate = setAppState(toUpdate, ind, installing, mcapp)
			if mcappState != "" {
				mcappState = installing
			}
		} else if !v33.AppConditionDeployed.IsTrue(app) {
			toUpdate = setAppState(toUpdate, ind, deploying, mcapp)
			if mcappState == active {
				mcappState = deploying
			}
		} else {
			toUpdate = setAppState(toUpdate, ind, active, mcapp)
		}
	}
	if mcappState == installing || mcappState == deploying {
		toUpdate = setMcAppStateUnknown(toUpdate, mcapp, mcappState)
	} else if mcappState == active {
		toUpdate = setMcappStateActive(toUpdate, mcapp)
	}
	if toUpdate != nil {
		return m.MultiClusterApps.Update(toUpdate)
	}
	return mcapp, nil
}

func setAppState(toUpdate *v3.MultiClusterApp, ind int, state string, mcapp *v3.MultiClusterApp) *v3.MultiClusterApp {
	if mcapp.Spec.Targets[ind].State == state {
		return toUpdate
	}
	if toUpdate == nil {
		toUpdate = mcapp.DeepCopy()
	}
	toUpdate.Spec.Targets[ind].State = state
	return toUpdate
}

func setMcappStateActive(toUpdate *v3.MultiClusterApp, mcapp *v3.MultiClusterApp) *v3.MultiClusterApp {
	if v32.MultiClusterAppConditionInstalled.IsTrue(mcapp) && v32.MultiClusterAppConditionDeployed.IsTrue(mcapp) {
		return toUpdate
	}
	if toUpdate == nil {
		toUpdate = mcapp.DeepCopy()
	}
	v32.MultiClusterAppConditionDeployed.True(toUpdate)
	v32.MultiClusterAppConditionInstalled.True(toUpdate)
	return toUpdate
}

func setMcAppStateUnknown(toUpdate *v3.MultiClusterApp, mcapp *v3.MultiClusterApp, state string) *v3.MultiClusterApp {
	cond := v32.MultiClusterAppConditionInstalled
	cleanCond := v32.MultiClusterAppConditionDeployed
	if state == deploying {
		cond = v32.MultiClusterAppConditionDeployed
		cleanCond = v32.MultiClusterAppConditionInstalled
	}
	if !cond.IsUnknown(mcapp) {
		if toUpdate == nil {
			toUpdate = mcapp.DeepCopy()
		}
		if cleanCond.IsUnknown(toUpdate) {
			cleanCond.True(toUpdate)
		}
		cond.Unknown(toUpdate)
	}
	return toUpdate
}
