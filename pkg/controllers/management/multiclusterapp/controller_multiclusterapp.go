package multiclusterapp

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/namespace"
	"github.com/rancher/rancher/pkg/types/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	globalScopeAnswersKey     = "global"
	creatorIDAnn              = "field.cattle.io/creatorId"
	MultiClusterAppIDSelector = "mcapp"
	mcAppLabel                = "io.cattle.field/multiClusterAppId"
)

type MCAppManager struct {
	apps                          pv3.AppInterface
	appLister                     pv3.AppLister
	multiClusterApps              v3.MultiClusterAppInterface
	multiClusterAppRevisions      v3.MultiClusterAppRevisionInterface
	multiClusterAppRevisionLister v3.MultiClusterAppRevisionLister
	templateVersionLister         v3.CatalogTemplateVersionLister
	projectLister                 v3.ProjectLister
	clusterLister                 v3.ClusterLister
	userManager                   user.Manager
	ctx                           context.Context
}

func StartMCAppManagementController(ctx context.Context, mgmt *config.ManagementContext, clusterManager *clustermanager.Manager) {
	management := mgmt.Management
	mcApps := management.MultiClusterApps("")
	m := &MCAppManager{
		ctx:                           ctx,
		apps:                          mgmt.Project.Apps(""),
		appLister:                     mgmt.Project.Apps("").Controller().Lister(),
		multiClusterApps:              mcApps,
		multiClusterAppRevisions:      management.MultiClusterAppRevisions(""),
		multiClusterAppRevisionLister: management.MultiClusterAppRevisions("").Controller().Lister(),
		projectLister:                 management.Projects("").Controller().Lister(),
		clusterLister:                 management.Clusters("").Controller().Lister(),
		templateVersionLister:         management.CatalogTemplateVersions("").Controller().Lister(),
		userManager:                   mgmt.UserManager,
	}
	mcAppTickerData = map[string]*IntervalData{}
	m.multiClusterApps.AddHandler(ctx, "multi-cluster-app-controller", m.sync)
}

func (m *MCAppManager) sync(key string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	if mcapp == nil || mcapp.DeletionTimestamp != nil {
		_, mcappName, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return mcapp, err
		}
		deleteContext(mcappName)
		return m.deleteApps(mcappName, mcapp)
	}

	// creatorID is actual user who created mcapp, to be used for mcapp revisions of this mcapp
	metaAccessor, err := meta.Accessor(mcapp)
	if err != nil {
		return mcapp, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[rbac.CreatorIDAnn]
	if !ok {
		return mcapp, fmt.Errorf("MultiClusterApp %v has no creatorId annotation. Cannot create apps for %v", metaAccessor.GetName(), mcapp.Name)
	}
	// systemUserName is creatorID for app, the username of the service account created for this multiclusterapp
	systemUser, err := m.userManager.EnsureUser(fmt.Sprintf("system://%s", mcapp.Name), "System account for Multiclusterapp "+mcapp.Name)
	if err != nil {
		return nil, err
	}
	systemUserName := systemUser.Name

	answerMap, err := m.createAnswerMap(mcapp.Spec.Answers)
	if err != nil {
		return mcapp, err
	}

	externalID, mcapp, err := m.getExternalID(mcapp)
	if err != nil {
		return mcapp, err
	}

	mcapp = mcapp.DeepCopy()
	if err := m.reconcileTargetsForDelete(mcapp); err != nil {
		return mcapp, err
	}

	changed, err := m.isChanged(mcapp)
	if err != nil {
		return mcapp, err
	}

	toUpdate := false
	if changed {
		toUpdate, err = m.toUpdate(mcapp)
		if err != nil {
			return mcapp, err
		}
	}

	batchSize := len(mcapp.Spec.Targets)
	if toUpdate && mcapp.Spec.UpgradeStrategy.RollingUpdate != nil {
		if mcapp.Spec.UpgradeStrategy.RollingUpdate.Interval != 0 {
			batchSize = mcapp.Spec.UpgradeStrategy.RollingUpdate.BatchSize
		}
	}

	resp, err := m.createApps(mcapp, externalID, answerMap, systemUserName, batchSize, toUpdate)
	if err != nil {
		return resp.object, err
	}

	if !changed {
		if mcapp.Status.RevisionName == "" {
			return m.setRevisionAndUpdate(mcapp, creatorID)
		}
		return mcapp, nil
	}

	if resp.count == len(mcapp.Spec.Targets) && v3.MultiClusterAppConditionInstalled.IsUnknown(mcapp) &&
		v3.MultiClusterAppConditionInstalled.GetMessage(mcapp) == "upgrading" {
		deleteContext(mcapp.Name)
		return m.setRevisionAndUpdate(mcapp, creatorID)
	}

	if !toUpdate || resp.remaining <= 0 {
		return mcapp, nil
	}

	for i, app := range resp.updateApps {
		if _, err := m.updateApp(app, answerMap, externalID, resp.projects[i]); err != nil {
			return mcapp, err
		}
		resp.remaining--
		if resp.remaining == 0 {
			break
		}
	}

	setInstalledUnknown(mcapp)
	upd, err := m.updateCondition(mcapp, setInstalledUnknown)
	if err != nil {
		return mcapp, err
	}
	storeContext(m.ctx, mcapp, m.multiClusterApps)
	return upd, err
}

type Response struct {
	object     *v3.MultiClusterApp
	projects   []string
	updateApps []*pv3.App
	remaining  int
	count      int
}

func (m *MCAppManager) createApps(mcapp *v3.MultiClusterApp, externalID string, answerMap map[string]map[string]string,
	creatorID string, batchSize int, toUpdate bool) (*Response, error) {

	var mcappToUpdate *v3.MultiClusterApp
	var updateApps []*pv3.App
	var projects []string

	ann := map[string]string{
		creatorIDAnn: creatorID,
	}
	set := labels.Set(map[string]string{MultiClusterAppIDSelector: mcapp.Name})

	resp := &Response{object: mcapp}

	updateBatchSize := batchSize
	count := 0

	// for all targets, create the App{} instance, so that helm controller App lifecycle can pick it up
	// only one app per project named mcapp-{{mcapp.Name}}
	for ind, t := range mcapp.Spec.Targets {
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return resp, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
		}
		projectNS := split[1]
		// check if this app already exists
		if t.AppName != "" {
			app, err := m.appLister.Get(projectNS, t.AppName)
			if err != nil || app == nil {
				return resp, fmt.Errorf("error %v getting app %s in %s", err, t.AppName, projectNS)
			}
			if val, ok := app.Labels[MultiClusterAppIDSelector]; !ok || val != mcapp.Name {
				return resp, fmt.Errorf("app %s in %s missing multi cluster app label", t.AppName, projectNS)
			}
			appUpdated := false
			if app.Spec.ExternalID == externalID {
				if reflect.DeepEqual(app.Spec.Answers, getAnswerMap(answerMap, t.ProjectName)) {
					appUpdated = true
				}
			}
			if appUpdated {
				count++
				if !pv3.AppConditionInstalled.IsTrue(app) || !pv3.AppConditionDeployed.IsTrue(app) {
					toUpdate = false
					updateApps = []*pv3.App{}
				}
				continue
			}
			if toUpdate && updateBatchSize > 0 {
				updateApps = append(updateApps, app)
				projects = append(projects, t.ProjectName)
				updateBatchSize--
			}
			continue
		}
		if batchSize > 0 {
			appName, mcapp, err := m.createApp(mcapp, answerMap, ann, set, projectNS, creatorID, externalID, t.ProjectName)
			if err != nil {
				return resp, fmt.Errorf("error %v in creating multiclusterapp: %v", err, mcapp)
			}
			if appName != "" {
				if mcappToUpdate == nil {
					mcappToUpdate = mcapp.DeepCopy()
				}
				mcappToUpdate.Spec.Targets[ind].AppName = appName
				batchSize--
				count++
			}
		}
	}

	if mcappToUpdate != nil && !reflect.DeepEqual(mcapp, mcappToUpdate) {
		upd, err := m.multiClusterApps.Update(mcappToUpdate)
		if err != nil {
			resp.object = mcappToUpdate
			return resp, err
		}
		resp.object = upd
	}

	resp.updateApps = updateApps
	resp.projects = projects
	resp.count = count
	resp.remaining = batchSize

	return resp, nil
}

func (m *MCAppManager) updateApp(app *pv3.App, answerMap map[string]map[string]string, externalID string, projectName string) (*pv3.App, error) {
	app = app.DeepCopy()
	app.Spec.Answers = getAnswerMap(answerMap, projectName)
	app.Spec.ExternalID = externalID
	updatedObj, err := m.apps.Update(app)
	if err != nil && apierrors.IsConflict(err) {
		_, projectNS := ref.Parse(projectName)
		for i := 0; i < 5; i++ {
			latestObj, err := m.apps.GetNamespaced(projectNS, app.Name, metav1.GetOptions{})
			if err != nil {
				return latestObj, err
			}
			latestToUpdate := latestObj.DeepCopy()
			latestToUpdate.Spec.Answers = getAnswerMap(answerMap, projectName)
			latestToUpdate.Spec.ExternalID = externalID
			updated, err := m.apps.Update(latestToUpdate)
			if err != nil && apierrors.IsConflict(err) {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			return updated, err
		}
		return app, err
	}
	return updatedObj, err
}

func (m *MCAppManager) createRevision(mcapp *v3.MultiClusterApp, creatorID string) (*v3.MultiClusterAppRevision, error) {
	ownerReference := metav1.OwnerReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       rbac.MultiClusterAppResource,
		Name:       mcapp.Name,
		UID:        mcapp.UID,
	}
	revision := &v3.MultiClusterAppRevision{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				creatorIDAnn: creatorID,
			},
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
	}
	revision.GenerateName = "mcapprevision-"
	revision.Labels = map[string]string{
		mcAppLabel: mcapp.Name,
	}
	revision.Answers = mcapp.Spec.Answers
	revision.TemplateVersionName = mcapp.Spec.TemplateVersionName
	revision.Namespace = namespace.GlobalNamespace
	return m.multiClusterAppRevisions.Create(revision)
}

func (m *MCAppManager) setRevisionAndUpdate(mcapp *v3.MultiClusterApp, creatorID string) (*v3.MultiClusterApp, error) {
	latestMcApp, err := m.multiClusterApps.GetNamespaced(namespace.GlobalNamespace, mcapp.Name, metav1.GetOptions{})
	if err != nil {
		return mcapp, err
	}
	if latestMcApp.Status.RevisionName != "" {
		currRevision, err := m.multiClusterAppRevisionLister.Get(namespace.GlobalNamespace, latestMcApp.Status.RevisionName)
		if err != nil {
			return mcapp, err
		}
		if currRevision.TemplateVersionName == mcapp.Spec.TemplateVersionName &&
			reflect.DeepEqual(currRevision.Answers, mcapp.Spec.Answers) {
			return mcapp, nil
		}
		mcapp = latestMcApp
	}
	setInstalledDone(mcapp)
	rev, err := m.createRevision(mcapp, creatorID)
	if err != nil {
		return mcapp, err
	}
	mcapp.Status.RevisionName = rev.Name
	return m.updateCondition(mcapp, setInstalledDone)
}

func (m *MCAppManager) isChanged(mcapp *v3.MultiClusterApp) (bool, error) {
	if mcapp.Status.RevisionName == "" {
		return false, nil
	}
	mcappRevision, err := m.multiClusterAppRevisionLister.Get(namespace.GlobalNamespace, mcapp.Status.RevisionName)
	if err != nil {
		return false, err
	}
	if mcapp.Spec.TemplateVersionName != mcappRevision.TemplateVersionName {
		return true, nil
	}
	if !reflect.DeepEqual(mcapp.Spec.Answers, mcappRevision.Answers) {
		return true, nil
	}
	return false, nil
}

func (m *MCAppManager) toUpdate(mcapp *v3.MultiClusterApp) (bool, error) {
	if v3.MultiClusterAppConditionInstalled.IsUnknown(mcapp) && v3.MultiClusterAppConditionInstalled.GetMessage(mcapp) == "upgrading" {
		lastUpdated, err := time.Parse(time.RFC3339, v3.MultiClusterAppConditionInstalled.GetLastUpdated(mcapp))
		if err != nil {
			return false, err
		}
		interval := 0
		if mcapp.Spec.UpgradeStrategy.RollingUpdate != nil {
			interval = mcapp.Spec.UpgradeStrategy.RollingUpdate.Interval
		}
		if time.Since(lastUpdated) < time.Duration(interval)*time.Second {
			return false, nil
		}
	}
	return true, nil
}

func (m *MCAppManager) createApp(mcapp *v3.MultiClusterApp, answerMap map[string]map[string]string, ann map[string]string,
	set map[string]string, projectNS string, creatorID string, externalID string, projectName string) (string, *v3.MultiClusterApp, error) {
	nsName := getAppNamespaceName(mcapp.Name, projectNS)
	app, err := m.appLister.Get(projectNS, nsName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", mcapp, err
		}
		toCreate := pv3.App{
			ObjectMeta: metav1.ObjectMeta{
				Name:        nsName,
				Namespace:   projectNS,
				Annotations: ann,
				Labels:      set,
			},
			Spec: pv3.AppSpec{
				ProjectName:         projectName,
				TargetNamespace:     nsName,
				ExternalID:          externalID,
				MultiClusterAppName: mcapp.Name,
				Answers:             getAnswerMap(answerMap, projectName),
				Wait:                mcapp.Spec.Wait,
				Timeout:             mcapp.Spec.Timeout,
			},
		}
		// Now create the App instance
		app, err = m.apps.Create(&toCreate)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return "", mcapp, err
		}
	}
	return app.Name, mcapp, nil
}

func getAnswerMap(answerMap map[string]map[string]string, projectName string) map[string]string {
	// find answers for this project, if not found then try finding for the cluster this project belongs to, else finally use the global scoped answer
	answers := map[string]string{}
	if len(answerMap) > 0 {
		if ans, ok := answerMap[projectName]; ok {
			return ans
		}
		// find the answers for the cluster of this project
		split := strings.SplitN(projectName, ":", 2)
		clusterName := split[0]
		if ans, ok := answerMap[clusterName]; ok {
			return ans
		}
		if ans, ok := answerMap[globalScopeAnswersKey]; ok {
			return ans
		}
	}
	return answers
}

// deleteApps finds all apps created by this multiclusterapp and deletes them
func (m *MCAppManager) deleteApps(mcAppName string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	// get all apps with label "multiClusterAppId" = name of this app
	appsToDelete := []*pv3.App{}
	set := labels.Set(map[string]string{MultiClusterAppIDSelector: mcAppName})
	var err error

	if mcapp == nil {
		appsToDelete, err = m.getAllApps(mcAppName)
		if err != nil {
			return nil, err
		}
	} else {
		for _, t := range mcapp.Spec.Targets {
			split := strings.SplitN(t.ProjectName, ":", 2)
			if len(split) != 2 {
				return mcapp, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
			}
			projectNS := split[1]
			apps, err := m.appLister.List(projectNS, set.AsSelector())
			if err != nil {
				return nil, err
			}
			appsToDelete = append(appsToDelete, apps...)
		}
	}
	if err := m.delete(appsToDelete); err != nil {
		return nil, err
	}
	return nil, nil
}

func (m *MCAppManager) getAllApps(mcAppName string) ([]*pv3.App, error) {
	// to get all apps, get all clusters first, then get all apps in all projects of all clusters
	allApps := []*pv3.App{}
	set := labels.Set(map[string]string{MultiClusterAppIDSelector: mcAppName})
	clusters, err := m.clusterLister.List("", labels.NewSelector())
	if err != nil {
		return allApps, err
	}
	for _, c := range clusters {
		projects, err := m.projectLister.List(c.Name, labels.NewSelector())
		if err != nil {
			return allApps, err
		}
		for _, p := range projects {
			apps, err := m.appLister.List(p.Name, set.AsSelector())
			if err != nil {
				return allApps, err
			}
			allApps = append(allApps, apps...)
		}
	}
	return allApps, err
}

func (m *MCAppManager) reconcileTargetsForDelete(mcapp *v3.MultiClusterApp) error {
	existingApps := map[string]bool{}
	set := labels.Set(map[string]string{MultiClusterAppIDSelector: mcapp.Name})
	for _, t := range mcapp.Spec.Targets {
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return fmt.Errorf("error in splitting project ID %v", t.ProjectName)
		}
		projectNS := split[1]
		apps, err := m.appLister.List(projectNS, set.AsSelector())
		if err != nil {
			return err
		}
		for _, app := range apps {
			existingApps[app.Namespace] = true
		}
	}
	allApps, err := m.getAllApps(mcapp.Name)
	if err != nil {
		return err
	}
	toDelete := []*pv3.App{}
	for _, app := range allApps {
		if _, ok := existingApps[app.Namespace]; !ok {
			toDelete = append(toDelete, app)
		}
	}
	if len(toDelete) > 0 {
		logrus.Debugf("deleting apps for mcapp %s toDelete %v", mcapp.Name, toDelete)
	}
	return m.delete(toDelete)
}

func (m *MCAppManager) delete(appsToDelete []*pv3.App) error {
	var g errgroup.Group
	for ind := range appsToDelete {
		app := appsToDelete[ind]
		g.Go(func() error {
			if err := m.apps.DeleteNamespaced(app.Namespace, app.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func (m *MCAppManager) updateCondition(mcappToUpdate *v3.MultiClusterApp, setCondition func(mcapp *v3.MultiClusterApp)) (*v3.MultiClusterApp, error) {
	updatedObj, err := m.multiClusterApps.Update(mcappToUpdate)
	if err != nil && apierrors.IsConflict(err) {
		// retry 5 times
		for i := 0; i < 5; i++ {
			latestMcApp, err := m.multiClusterApps.GetNamespaced(namespace.GlobalNamespace, mcappToUpdate.Name, metav1.GetOptions{})
			if err != nil {
				return latestMcApp, err
			}
			latestToUpdate := latestMcApp.DeepCopy()
			for ind, t := range mcappToUpdate.Spec.Targets {
				if t.AppName != "" {
					latestToUpdate.Spec.Targets[ind].AppName = t.AppName
				}
			}
			latestToUpdate.Status.RevisionName = mcappToUpdate.Status.RevisionName
			setCondition(latestToUpdate)
			updatedMcApp, err := m.multiClusterApps.Update(latestToUpdate)
			if err != nil && apierrors.IsConflict(err) {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			return updatedMcApp, err
		}
		return mcappToUpdate, err
	}
	return updatedObj, err
}

func setInstalledUnknown(mcapp *v3.MultiClusterApp) {
	v3.MultiClusterAppConditionInstalled.Unknown(mcapp)
	v3.MultiClusterAppConditionInstalled.Message(mcapp, "upgrading")
	v3.MultiClusterAppConditionInstalled.LastUpdated(mcapp, time.Now().Format(time.RFC3339))
}

func setInstalledDone(mcapp *v3.MultiClusterApp) {
	v3.MultiClusterAppConditionInstalled.True(mcapp)
	v3.MultiClusterAppConditionInstalled.Message(mcapp, "")
}

func (m *MCAppManager) createAnswerMap(answers []v3.Answer) (map[string]map[string]string, error) {
	// create a map, where key is the projectID or clusterID, or "global" if neither is provided, and value is the actual answer values
	// Global scoped answers will have all questions. Project/cluster scoped will only have override keys. So we'll first create a global map,
	// and then merge with project/cluster map
	answerMap := make(map[string]map[string]string)
	globalAnswersMap := make(map[string]string)
	for _, a := range answers {
		if a.ProjectName == "" && a.ClusterName == "" {
			globalAnswersMap = a.Values
			answerMap[globalScopeAnswersKey] = make(map[string]string)
			answerMap[globalScopeAnswersKey] = a.Values
		}
	}

	for _, a := range answers {
		if a.ClusterName != "" {
			// Using k8s labels.Merge, since by definition:
			// Merge combines given maps, and does not check for any conflicts between the maps. In case of conflicts, second map (labels2) wins
			// And we want cluster level keys to override keys from global/cluster for that cluster
			clusterLabels := labels.Merge(globalAnswersMap, a.Values)
			answerMap[a.ClusterName] = make(map[string]string)
			answerMap[a.ClusterName] = clusterLabels
		}
	}

	for _, a := range answers {
		if a.ProjectName != "" {
			// check if answers for the cluster of this project are provided
			split := strings.SplitN(a.ProjectName, ":", 2)
			if len(split) != 2 {
				return answerMap, fmt.Errorf("error in splitting project name: %v", a.ProjectName)
			}
			clusterName := split[0]
			// Using k8s labels.Merge, since by definition:
			// Merge combines given maps, and does not check for any conflicts between the maps. In case of conflicts, second map (labels2) wins
			// And we want project level keys to override keys from global level for that project
			projectLabels := make(map[string]string)
			if val, ok := answerMap[clusterName]; ok {
				projectLabels = labels.Merge(val, a.Values)
			} else {
				projectLabels = labels.Merge(globalAnswersMap, a.Values)
			}
			answerMap[a.ProjectName] = make(map[string]string)
			answerMap[a.ProjectName] = projectLabels
		}
	}

	return answerMap, nil
}

// getExternalID gets the TemplateVersion.Spec.ExternalID field
func (m *MCAppManager) getExternalID(mcapp *v3.MultiClusterApp) (string, *v3.MultiClusterApp, error) {
	// create the externalID field, it's also present on the templateVersion. So get the templateVersion and read its externalID field
	split := strings.SplitN(mcapp.Spec.TemplateVersionName, ":", 2)
	templateVersionNamespace := split[0]
	templateVersionName := split[1]
	tv, err := m.templateVersionLister.Get(templateVersionNamespace, templateVersionName)
	if err != nil {
		return "", mcapp, err
	}
	if tv == nil {
		return "", mcapp, fmt.Errorf("invalid templateVersion provided: %v", mcapp.Spec.TemplateVersionName)
	}

	externalID := tv.Spec.ExternalID
	return externalID, mcapp, nil
}

func getAppNamespaceName(mcappName, projectNS string) string {
	return fmt.Sprintf("%s-%s", mcappName, projectNS)
}
