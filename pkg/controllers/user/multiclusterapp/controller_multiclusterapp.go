package multiclusterapp

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
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
	globalNamespace           = "cattle-global-data"
)

type MCAppController struct {
	apps                  pv3.AppInterface
	appLister             pv3.AppLister
	multiClusterApps      v3.MultiClusterAppInterface
	multiClusterAppLister v3.MultiClusterAppLister
	namespaces            corev1.NamespaceInterface
	templateVersionLister v3.TemplateVersionLister
	clusterLister         v3.ClusterLister
	projectLister         v3.ProjectLister
	clusterName           string
}

func Register(ctx context.Context, cluster *config.UserContext) {
	m := &MCAppController{
		apps:                  cluster.Management.Project.Apps(""),
		appLister:             cluster.Management.Project.Apps("").Controller().Lister(),
		namespaces:            cluster.Core.Namespaces(""),
		multiClusterApps:      cluster.Management.Management.MultiClusterApps(""),
		multiClusterAppLister: cluster.Management.Management.MultiClusterApps("").Controller().Lister(),
		clusterLister:         cluster.Management.Management.Clusters("").Controller().Lister(),
		projectLister:         cluster.Management.Management.Projects("").Controller().Lister(),
		clusterName:           cluster.ClusterName,
		templateVersionLister: cluster.Management.Management.TemplateVersions("").Controller().Lister(),
	}
	m.multiClusterApps.AddHandler(ctx, "multi-cluster-app-controller", m.sync)
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

	// for all targets, create the App{} instance, so that helm controller App lifecycle can pick it up
	// only one app per project named mcapp-{{mcapp.Name}}
	var mcappToUpdate *v3.MultiClusterApp
	ann := make(map[string]string)
	ann[creatorIDAnn] = creatorID
	set := labels.Set(map[string]string{multiClusterAppIDSelector: mcapp.Name})
	for ind, t := range mcapp.Spec.Targets {
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return mcapp, fmt.Errorf("error %v in splitting project ID %v", err, t.ProjectName)
		}
		projectNS := split[1]
		// check if the target project in this iteration is same as the cluster in current context, if not then don't create namespace and app else it
		// will be in the wrong cluster
		if split[0] != m.clusterName {
			logrus.Debugf("Not for the current cluster since project %v doesn't belong in cluster %v", split[1], m.clusterName)
			continue
		}

		// check if this app already exists
		a, err := m.appLister.List(projectNS, set.AsSelector())
		if err != nil {
			return nil, fmt.Errorf("Error %v in listing apps using selector in project %v", err, projectNS)
		}
		if len(a) > 0 {
			logrus.Debugf("App for multiclusterapp %v already exists in project %v", mcapp.Name, t.ProjectName)
			continue
		}
		// call createNsAndApp method
		newTarget, mcapp, err := m.createNamespaceAndApp(&t, mcapp, answerMap, ann, set, projectNS, creatorID, externalID)
		if err != nil {
			return mcapp, fmt.Errorf("error %v in creating multiclusterapp: %v", err, mcapp)
		}
		if newTarget != nil {
			mcappToUpdate = mcapp.DeepCopy()
			mcappToUpdate.Spec.Targets[ind].AppName = newTarget.AppName
		}
	}

	if mcappToUpdate != nil && !reflect.DeepEqual(mcapp, mcappToUpdate) {
		return m.update(mcappToUpdate)
	}
	return mcapp, nil
}

// createNamespaceAndApp creates the namespace for all workloads of the app, and then the app itself
func (m *MCAppController) createNamespaceAndApp(t *v3.Target, mcapp *v3.MultiClusterApp, answerMap map[string]map[string]string, ann map[string]string,
	set map[string]string, projectNS string, creatorID string, externalID string) (*v3.Target, *v3.MultiClusterApp, error) {
	var answerFound bool
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

	// find answers for this project, if not found then try finding for the cluster this project belongs to, else finally use the global scoped answer
	if len(answerMap) > 0 {
		if ans, ok := answerMap[t.ProjectName]; ok {
			app.Spec.Answers = ans
			answerFound = true
		}
		if !answerFound {
			// find the answers for the cluster of this project
			split := strings.SplitN(t.ProjectName, ":", 2)
			if len(split) != 2 {
				return nil, mcapp, fmt.Errorf("error in splitting project name: %v", t.ProjectName)
			}
			clusterName := split[0]
			if ans, ok := answerMap[clusterName]; ok {
				app.Spec.Answers = ans
				answerFound = true
			}
			if !answerFound {
				if ans, ok := answerMap[globalScopeAnswersKey]; ok {
					app.Spec.Answers = ans
				}
			}
		}
	}
	// Now create the App instance
	createdApp, err := m.apps.Create(&app)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, mcapp, err
	}
	// App creation is successful, so set Target.AppID = createdApp.Name
	t.AppName = createdApp.Name
	return t, mcapp, nil
}

// deleteApps finds all apps created by this multiclusterapp and deletes them
func (m *MCAppController) deleteApps(mcAppName string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	// get all apps with label "multiClusterAppId" = name of this app
	appsToDelete := []*pv3.App{}
	set := labels.Set(map[string]string{multiClusterAppIDSelector: mcAppName})
	var err error

	if mcapp == nil {
		appsToDelete, _, err = m.getAllAppsToDelete(mcAppName)
		if err != nil {
			return nil, err
		}
	} else {
		for _, t := range mcapp.Spec.Targets {
			apps, err := m.appLister.List(t.ProjectName, set.AsSelector())
			if err != nil {
				return nil, err
			}
			appsToDelete = append(appsToDelete, apps...)
		}
	}

	var g errgroup.Group
	for _, app := range appsToDelete {
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
		return nil, err
	}
	return nil, nil
}

func (m *MCAppController) getAllAppsToDelete(mcAppName string) ([]*pv3.App, *v3.MultiClusterApp, error) {
	// to get all apps, get all clusters first, then get all apps in all projects of all clusters
	appsToDelete := []*pv3.App{}
	set := labels.Set(map[string]string{multiClusterAppIDSelector: mcAppName})
	clusters, err := m.clusterLister.List("", labels.NewSelector())
	if err != nil {
		return appsToDelete, nil, err
	}
	for _, c := range clusters {
		projects, err := m.projectLister.List(c.Name, labels.NewSelector())
		if err != nil {
			return appsToDelete, nil, err
		}
		for _, p := range projects {
			apps, err := m.appLister.List(p.Name, set.AsSelector())
			if err != nil {
				return appsToDelete, nil, err
			}
			appsToDelete = append(appsToDelete, apps...)
		}
	}
	return appsToDelete, nil, err
}

func (m *MCAppController) update(mcappToUpdate *v3.MultiClusterApp) (*v3.MultiClusterApp, error) {
	updatedObj, err := m.multiClusterApps.Update(mcappToUpdate)
	if err != nil && apierrors.IsConflict(err) {
		// retry 5 times
		for i := 0; i < 5; i++ {
			latestMcApp, err := m.multiClusterApps.GetNamespaced(globalNamespace, mcappToUpdate.Name, metav1.GetOptions{})
			if err != nil {
				return latestMcApp, err
			}
			latestToUpdate := latestMcApp.DeepCopy()
			for ind, t := range mcappToUpdate.Spec.Targets {
				if t.AppName != "" {
					latestToUpdate.Spec.Targets[ind].AppName = t.AppName
				}
			}
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
	tv, err := m.templateVersionLister.Get("", mcapp.Spec.TemplateVersionName)
	if err != nil {
		return "", mcapp, err
	}
	if tv == nil {
		return "", mcapp, fmt.Errorf("Invalid templateVersion provided: %v", mcapp.Spec.TemplateVersionName)
	}

	externalID := tv.Spec.ExternalID
	return externalID, mcapp, nil
}
