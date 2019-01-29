package multiclusterapp

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/management/globalnamespacerbac"
	"github.com/rancher/rancher/pkg/ref"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/namespace"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"golang.org/x/sync/errgroup"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	globalScopeAnswersKey     = "global"
	creatorIDAnn              = "field.cattle.io/creatorId"
	multiClusterAppIDSelector = "mcapp"
	projectIDFieldLabel       = "field.cattle.io/projectId"
	mcAppLabel                = "io.cattle.field/multiClusterAppId"
)

type MCAppController struct {
	apps                          pv3.AppInterface
	appLister                     pv3.AppLister
	multiClusterApps              v3.MultiClusterAppInterface
	multiClusterAppLister         v3.MultiClusterAppLister
	multiClusterAppRevisions      v3.MultiClusterAppRevisionInterface
	multiClusterAppRevisionLister v3.MultiClusterAppRevisionLister
	namespaces                    corev1.NamespaceInterface
	templateVersionLister         v3.CatalogTemplateVersionLister
	clusterLister                 v3.ClusterLister
	projectLister                 v3.ProjectLister
	clusterName                   string
}

func Register(ctx context.Context, cluster *config.UserContext) {
	mcApps := cluster.Management.Management.MultiClusterApps("")
	m := &MCAppController{
		apps:                          cluster.Management.Project.Apps(""),
		appLister:                     cluster.Management.Project.Apps("").Controller().Lister(),
		namespaces:                    cluster.Core.Namespaces(""),
		multiClusterApps:              mcApps,
		multiClusterAppLister:         mcApps.Controller().Lister(),
		multiClusterAppRevisions:      cluster.Management.Management.MultiClusterAppRevisions(""),
		multiClusterAppRevisionLister: cluster.Management.Management.MultiClusterAppRevisions("").Controller().Lister(),
		clusterLister:                 cluster.Management.Management.Clusters("").Controller().Lister(),
		projectLister:                 cluster.Management.Management.Projects("").Controller().Lister(),
		clusterName:                   cluster.ClusterName,
		templateVersionLister:         cluster.Management.Management.CatalogTemplateVersions("").Controller().Lister(),
	}
	m.multiClusterApps.AddHandler(ctx, "multi-cluster-app-controller", m.sync)

	StartMCAppStateController(ctx, cluster)
}

func (m *MCAppController) sync(key string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	if mcapp == nil || mcapp.DeletionTimestamp != nil {
		_, mcappName, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return mcapp, err
		}
		return m.deleteApps(mcappName, mcapp)
	}
	metaAccessor, err := meta.Accessor(mcapp)
	if err != nil {
		return mcapp, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
	if !ok {
		return mcapp, fmt.Errorf("MultiClusterApp %v has no creatorId annotation. Cannot create apps for %v", metaAccessor.GetName(), mcapp.Name)
	}
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

	toUpgrade, err := m.toUpgrade(mcapp)
	if err != nil {
		return mcapp, err
	}
	batchSize := len(mcapp.Spec.Targets)
	if toUpgrade && mcapp.Spec.UpgradeStrategy.RollingUpdate != nil {
		batchSize = mcapp.Spec.UpgradeStrategy.RollingUpdate.BatchSize
	}

	// todo: need to make this generic so works for other upgrade strategies
	resp, err := m.createApps(mcapp, externalID, answerMap, creatorID, batchSize, toUpgrade)
	if err != nil {
		return resp.object, err
	}

	if !toUpgrade {
		if mcapp.Status.RevisionName == "" {
			setInstalledDone(mcapp)
			return m.setRevisionAndUpdate(mcapp, creatorID)
		}
		return mcapp, nil
	}

	if !resp.canProceed {
		return mcapp, nil
	}

	if len(resp.updateApps) == 0 && v3.MultiClusterAppConditionInstalled.IsUnknown(mcapp) {
		setInstalledDone(mcapp)
		return m.setRevisionAndUpdate(mcapp, creatorID)
	}

	if resp.remaining == 0 || len(resp.updateApps) == 0 {
		return mcapp, nil
	}

	for i, app := range resp.updateApps {
		if resp.remaining > 0 {
			if _, err := m.updateApp(app, answerMap, externalID, resp.projects[i]); err != nil {
				return mcapp, err
			}
			resp.remaining--
		}
	}

	setInstalledUnknown(mcapp)
	return m.updateCondition(mcapp, setInstalledUnknown)
}

type Response struct {
	object     *v3.MultiClusterApp
	updateApps []*pv3.App
	projects   []string
	remaining  int
	canProceed bool
}

func (m *MCAppController) createApps(mcapp *v3.MultiClusterApp, externalID string, answerMap map[string]map[string]string,
	creatorID string, batchSize int, toUpdate bool) (*Response, error) {

	var mcappToUpdate *v3.MultiClusterApp
	var updateApps []*pv3.App
	var projects []string

	ann := map[string]string{
		creatorIDAnn: creatorID,
	}
	set := labels.Set(map[string]string{multiClusterAppIDSelector: mcapp.Name})

	resp := &Response{object: mcapp}
	updateBatchSize := batchSize

	// for all targets, create the App{} instance, so that helm controller App lifecycle can pick it up
	// only one app per project named mcapp-{{mcapp.Name}}
	for ind, t := range mcapp.Spec.Targets {
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return resp, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
		}
		projectNS := split[1]
		// check if the target project in this iteration is same as the cluster in current context, if not then don't create namespace and app else it
		// will be in the wrong cluster
		if split[0] != m.clusterName {
			continue
		}
		// check if this app already exists
		if t.AppName != "" {
			app, err := m.appLister.Get(projectNS, t.AppName)
			if err != nil || app == nil {
				return resp, fmt.Errorf("error %v getting app %s in %s", err, t.AppName, projectNS)
			}
			if val, ok := app.Labels[multiClusterAppIDSelector]; !ok || val != mcapp.Name {
				return resp, fmt.Errorf("app %s in %s missing multi cluster app label", t.AppName, projectNS)
			}
			if toUpdate && updateBatchSize > 0 {
				appUpdated := false
				if app.Spec.ExternalID == externalID {
					if reflect.DeepEqual(app.Spec.Answers, getAnswerMap(answerMap, t.ProjectName)) {
						appUpdated = true
					}
				}
				if appUpdated {
					if !pv3.AppConditionInstalled.IsTrue(app) || !pv3.AppConditionDeployed.IsTrue(app) {
						toUpdate = false
						updateApps = []*pv3.App{}
					}
					continue
				}
				updateApps = append(updateApps, app)
				projects = append(projects, t.ProjectName)
				updateBatchSize--
			}
			continue
		}
		if batchSize > 0 {
			newTarget, mcapp, err := m.createNamespaceAndApp(&t, mcapp, answerMap, ann, set, projectNS, creatorID, externalID)
			if err != nil {
				return resp, fmt.Errorf("error %v in creating multiclusterapp: %v", err, mcapp)
			}
			if newTarget != nil {
				if mcappToUpdate == nil {
					mcappToUpdate = mcapp.DeepCopy()
				}
				mcappToUpdate.Spec.Targets[ind].AppName = newTarget.AppName
			}
			batchSize--
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
	resp.canProceed = toUpdate
	resp.projects = projects
	resp.remaining = batchSize

	return resp, nil
}

func (m *MCAppController) updateApp(app *pv3.App, answerMap map[string]map[string]string, externalID string, projectName string) (*pv3.App, error) {
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

func (m *MCAppController) createRevision(mcapp *v3.MultiClusterApp, creatorID string) (*v3.MultiClusterAppRevision, error) {
	ownerReference := metav1.OwnerReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       globalnamespacerbac.MultiClusterAppResource,
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

func (m *MCAppController) setRevisionAndUpdate(mcapp *v3.MultiClusterApp, creatorID string) (*v3.MultiClusterApp, error) {
	rev, err := m.createRevision(mcapp, creatorID)
	if err != nil {
		return mcapp, err
	}
	mcapp.Status.RevisionName = rev.Name
	return m.updateCondition(mcapp, setInstalledDone)
}

func (m *MCAppController) toUpgrade(mcapp *v3.MultiClusterApp) (bool, error) {
	if mcapp.Status.RevisionName == "" {
		return false, nil
	}
	mcappRevision, err := m.multiClusterAppRevisionLister.Get(namespace.GlobalNamespace, mcapp.Status.RevisionName)
	if err != nil {
		return false, err
	}
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
	if mcapp.Spec.TemplateVersionName != mcappRevision.TemplateVersionName {
		return true, nil
	}
	if !reflect.DeepEqual(mcapp.Spec.Answers, mcappRevision.Answers) {
		return true, nil
	}
	return false, nil
}

// createNamespaceAndApp creates the namespace for all workloads of the app, and then the app itself
func (m *MCAppController) createNamespaceAndApp(t *v3.Target, mcapp *v3.MultiClusterApp, answerMap map[string]map[string]string, ann map[string]string,
	set map[string]string, projectNS string, creatorID string, externalID string) (*v3.Target, *v3.MultiClusterApp, error) {
	// Create the target namespace first
	// Adding the projectId as an annotation is necessary, else the API/UI and UI won't list any of the resources from this namespace
	n := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: mcapp.Name + "-",
			Labels:       map[string]string{projectIDFieldLabel: projectNS},
			Annotations:  map[string]string{projectIDFieldLabel: t.ProjectName, creatorIDAnn: creatorID},
		},
	}
	ns, err := m.namespaces.Create(&n)
	if err != nil {
		return nil, mcapp, err
	}

	app := pv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ns.Name,
			Namespace:   projectNS,
			Annotations: ann,
			Labels:      set,
		},
		Spec: pv3.AppSpec{
			ProjectName:         t.ProjectName,
			TargetNamespace:     ns.Name,
			ExternalID:          externalID,
			MultiClusterAppName: mcapp.Name,
		},
	}

	app.Spec.Answers = getAnswerMap(answerMap, t.ProjectName)
	// Now create the App instance
	createdApp, err := m.apps.Create(&app)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, mcapp, err
	}
	// App creation is successful, so set Target.AppID = createdApp.Name
	t.AppName = createdApp.Name
	return t, mcapp, nil
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
func (m *MCAppController) deleteApps(mcAppName string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	// get all apps with label "multiClusterAppId" = name of this app
	appsToDelete := []*pv3.App{}
	set := labels.Set(map[string]string{multiClusterAppIDSelector: mcAppName})
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

func (m *MCAppController) getAllApps(mcAppName string) ([]*pv3.App, error) {
	// to get all apps, get all clusters first, then get all apps in all projects of all clusters
	appsToDelete := []*pv3.App{}
	set := labels.Set(map[string]string{multiClusterAppIDSelector: mcAppName})
	clusters, err := m.clusterLister.List("", labels.NewSelector())
	if err != nil {
		return appsToDelete, err
	}
	for _, c := range clusters {
		projects, err := m.projectLister.List(c.Name, labels.NewSelector())
		if err != nil {
			return appsToDelete, err
		}
		for _, p := range projects {
			apps, err := m.appLister.List(p.Name, set.AsSelector())
			if err != nil {
				return appsToDelete, err
			}
			appsToDelete = append(appsToDelete, apps...)
		}
	}
	return appsToDelete, err
}

func (m *MCAppController) reconcileTargetsForDelete(mcapp *v3.MultiClusterApp) error {
	existingApps := map[string][]string{}
	for ind, t := range mcapp.Spec.Targets {
		if t.AppName == "" {
			continue
		}
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return fmt.Errorf("invalid project name %s", t.ProjectName)
		}
		existingApps[t.AppName] = []string{split[1], strconv.Itoa(ind)}
	}
	allApps, err := m.getAllApps(mcapp.Name)
	if err != nil {
		return err
	}
	toDelete := []*pv3.App{}
	for _, app := range allApps {
		val, ok := existingApps[app.Name]
		if !ok {
			toDelete = append(toDelete, app)
		} else if val[0] != app.Namespace {
			toDelete = append(toDelete, app)
			ind, err := strconv.Atoi(val[1])
			if err != nil {
				return err
			}
			mcapp.Spec.Targets[ind].AppName = ""
		}
	}
	return m.delete(toDelete)
}

func (m *MCAppController) delete(appsToDelete []*pv3.App) error {
	var g errgroup.Group
	for ind := range appsToDelete {
		app := appsToDelete[ind]
		g.Go(func() error {
			var appWorkloadNamespace string
			if app != nil {
				appWorkloadNamespace = app.Spec.TargetNamespace
			}
			if err := m.apps.DeleteNamespaced(app.Namespace, app.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if err := m.namespaces.Delete(appWorkloadNamespace, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
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

func (m *MCAppController) updateCondition(mcappToUpdate *v3.MultiClusterApp, setCondition func(mcapp *v3.MultiClusterApp)) (*v3.MultiClusterApp, error) {
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

func (m *MCAppController) createAnswerMap(answers []v3.Answer) (map[string]map[string]string, error) {
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
			// And we want cluster level keys to override keys from global level for that cluster
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
func (m *MCAppController) getExternalID(mcapp *v3.MultiClusterApp) (string, *v3.MultiClusterApp, error) {
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
