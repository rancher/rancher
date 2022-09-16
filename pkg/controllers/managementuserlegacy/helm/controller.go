package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	errorsutil "github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose/common"
	hCommon "github.com/rancher/rancher/pkg/controllers/managementuserlegacy/helm/common"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	helmTokenPrefix           = "helm-token-"
	description               = "token for helm chart deployment"
	AppIDsLabel               = "cattle.io/appIds"
	creatorIDAnn              = "field.cattle.io/creatorId"
	MultiClusterAppIDSelector = "mcapp"
	projectIDFieldLabel       = "field.cattle.io/projectId"
	defaultMaxRevisionCount   = 10
)

func Register(ctx context.Context, mgmt *config.ScaledContext, user *config.UserContext, kubeConfigGetter common.KubeConfigGetter) {
	starter := user.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, mgmt, user, kubeConfigGetter)
		return nil
	})

	apps := user.Management.Project.Apps("")
	apps.AddClusterScopedHandler(ctx, "helm-deferred", user.ClusterName, func(key string, obj *v32.App) (runtime.Object, error) {
		return obj, starter()
	})
}

func registerDeferred(ctx context.Context, mgmt *config.ScaledContext, user *config.UserContext, kubeConfigGetter common.KubeConfigGetter) {
	appClient := user.Management.Project.Apps("")
	stackLifecycle := &Lifecycle{
		KubeConfigGetter:      kubeConfigGetter,
		SystemAccountManager:  systemaccount.NewManager(user.Management),
		TokenClient:           user.Management.Management.Tokens(""),
		UserClient:            user.Management.Management.Users(""),
		K8sClient:             user.K8sClient,
		TemplateVersionClient: user.Management.Management.CatalogTemplateVersions(""),
		TemplateClient:        user.Management.Management.CatalogTemplates(""),
		CatalogLister:         user.Management.Management.Catalogs("").Controller().Lister(),
		ClusterCatalogLister:  user.Management.Management.ClusterCatalogs("").Controller().Lister(),
		ProjectCatalogLister:  user.Management.Management.ProjectCatalogs("").Controller().Lister(),
		TemplateVersionLister: user.Management.Management.CatalogTemplateVersions("").Controller().Lister(),
		ClusterName:           user.ClusterName,
		systemTokens:          user.Management.SystemTokens,
		AppRevisionGetter:     user.Management.Project,
		AppGetter:             user.Management.Project,
		AppsLister:            user.Management.Project.Apps("").Controller().Lister(),
		NsLister:              user.Core.Namespaces("").Controller().Lister(),
		NsClient:              user.Core.Namespaces(""),
		SecretLister:          mgmt.Core.Secrets("").Controller().Lister(),
	}
	appClient.AddClusterScopedLifecycle(ctx, "helm-controller", user.ClusterName, stackLifecycle)

	StartStateCalculator(ctx, user)
}

type Lifecycle struct {
	systemTokens          systemtokens.Interface
	KubeConfigGetter      common.KubeConfigGetter
	SystemAccountManager  *systemaccount.Manager
	UserManager           user.Manager
	TokenClient           mgmtv3.TokenInterface
	UserClient            mgmtv3.UserInterface
	TemplateVersionClient mgmtv3.CatalogTemplateVersionInterface
	TemplateClient        mgmtv3.CatalogTemplateInterface
	CatalogLister         mgmtv3.CatalogLister
	ClusterCatalogLister  mgmtv3.ClusterCatalogLister
	ProjectCatalogLister  mgmtv3.ProjectCatalogLister
	TemplateVersionLister mgmtv3.CatalogTemplateVersionLister
	K8sClient             kubernetes.Interface
	ClusterName           string
	AppRevisionGetter     v3.AppRevisionsGetter
	AppGetter             v3.AppsGetter
	AppsLister            v3.AppLister
	NsLister              corev1.NamespaceLister
	NsClient              corev1.NamespaceInterface
	SecretLister          corev1.SecretLister
}

func (l *Lifecycle) Create(obj *v3.App) (runtime.Object, error) {
	v32.AppConditionMigrated.True(obj)
	v32.AppConditionUserTriggeredAction.Unknown(obj)
	if obj.Spec.ExternalID != "" {
		helmVersion, err := l.getHelmVersion(obj)
		if err != nil {
			return nil, err
		}
		obj.Status.HelmVersion = helmVersion
	}

	return obj, nil
}

/*
Updated depends on several conditions:

	AppConditionMigrated: protects upgrade path for apps <2.1
	AppConditionInstalled: flips status in UI and drives logic
	AppConditionDeployed: flips status in UI
	AppConditionForceUpgrade: add destructive `--force` param to helm upgrade when set to Unknown
	AppConditionUserTriggeredAction: Indicates when `upgrade` or `rollback` is called by the user
*/
func (l *Lifecycle) Updated(obj *v3.App) (runtime.Object, error) {
	if obj.Spec.ExternalID == "" && len(obj.Spec.Files) == 0 {
		return obj, nil
	}
	// always refresh app to avoid updating app twice
	_, projectName := ref.Parse(obj.Spec.ProjectName)
	var err error
	obj, err = l.AppsLister.Get(projectName, obj.Name)
	if err != nil {
		return obj, err
	}
	// if app was created before 2.1, run migrate to install a no-op helm release
	newObj, err := v32.AppConditionMigrated.Once(obj, func() (runtime.Object, error) {
		return l.DeployApp(obj)
	})
	if err != nil {
		return obj, err
	}

	obj = newObj.(*v3.App)
	appRevisionClient := l.AppRevisionGetter.AppRevisions(projectName)
	if obj.Spec.AppRevisionName != "" {
		currentRevision, err := appRevisionClient.Get(obj.Spec.AppRevisionName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		/*
			This if statement gates most of the logic for Update. We should only deploy the app when we actually want to.
			But we call update when we may only want to change a UI status. In those cases, we should return here and not
			go further. We want to deploy the app when:
				* The current App revision is different than the app, ex. the app is different than it was before
				* The force upgrade flag is set, where AppConditionForceUpgrade being unknown is equal to true.
				* The user caused the action by way of clicking either upgrade or rollback
		*/
		if isSame(obj, currentRevision) && !v32.AppConditionForceUpgrade.IsUnknown(obj) &&
			!v32.AppConditionUserTriggeredAction.IsTrue(obj) {
			if !v32.AppConditionForceUpgrade.IsTrue(obj) {
				v32.AppConditionForceUpgrade.True(obj)
			}
			logrus.Debugf("[helm-controller] App %v doesn't require update", obj.Name)
			return obj, nil
		}
	}
	logrus.Debugf("[helm-controller] Updating app %v", obj.Name)
	created := false
	if obj.Spec.MultiClusterAppName != "" {
		if _, err := l.NsLister.Get("", obj.Spec.TargetNamespace); err != nil && !errors.IsNotFound(err) {
			return obj, err
		} else if err != nil && errors.IsNotFound(err) {
			n := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: obj.Spec.TargetNamespace,
					Labels: map[string]string{
						projectIDFieldLabel:       obj.Namespace,
						MultiClusterAppIDSelector: obj.Spec.MultiClusterAppName},
					Annotations: map[string]string{
						projectIDFieldLabel: fmt.Sprintf("%s:%s", l.ClusterName, obj.Namespace),
						creatorIDAnn:        obj.Annotations[creatorIDAnn],
						AppIDsLabel:         obj.Name},
				},
			}
			ns, err := l.NsClient.Create(&n)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return obj, err
				}
				if ns.DeletionTimestamp != nil {
					return obj, fmt.Errorf("waiting for namespace %s to be terminated", obj.Namespace)
				}
			} else if err == nil {
				created = true
			}
		}
	}
	result, err := l.DeployApp(obj)
	if err != nil {
		return result, err
	}
	if !v32.AppConditionForceUpgrade.IsTrue(obj) {
		v32.AppConditionForceUpgrade.True(obj)
	}
	ns, err := l.NsClient.Get(obj.Spec.TargetNamespace, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return result, err
		}
		if obj.Spec.MultiClusterAppName != "" {
			return obj, fmt.Errorf("namespace not found after app creation %s", obj.Name)
		}
		return result, nil
	}
	if created {
		return result, nil
	}
	if ns.Annotations[AppIDsLabel] == "" {
		ns.Annotations[AppIDsLabel] = obj.Name
	} else {
		parts := strings.Split(ns.Annotations[AppIDsLabel], ",")
		appIDs := map[string]struct{}{}
		for _, part := range parts {
			appIDs[part] = struct{}{}
		}
		appIDs[obj.Name] = struct{}{}
		appIDList := []string{}
		for k := range appIDs {
			appIDList = append(appIDList, k)
		}
		sort.Strings(appIDList)
		ns.Annotations[AppIDsLabel] = strings.Join(appIDList, ",")
	}
	if _, err := l.NsClient.Update(ns); err != nil {
		return result, err
	}
	return result, nil
}

func (l *Lifecycle) DeployApp(obj *v3.App) (*v3.App, error) {
	obj = obj.DeepCopy()
	var err error
	if v32.AppConditionInstalled.IsTrue(obj) {
		v32.AppConditionInstalled.Unknown(obj)
		// update status in the UI
		obj, err = l.AppGetter.Apps("").Update(obj)
		if err != nil {
			return obj, err
		}
	}
	newObj, err := v32.AppConditionInstalled.Do(obj, func() (runtime.Object, error) {
		template, tempDirs, err := l.generateTemplates(obj)
		if err != nil {
			return obj, err
		}
		defer os.RemoveAll(tempDirs.FullPath)
		if err := l.Run(obj, template, tempDirs); err != nil {
			return obj, err
		}
		return obj, nil
	})
	return newObj.(*v3.App), err
}

func (l *Lifecycle) Remove(obj *v3.App) (runtime.Object, error) {
	_, projectName := ref.Parse(obj.Spec.ProjectName)
	appRevisionClient := l.AppRevisionGetter.AppRevisions(projectName)
	revisions, err := appRevisionClient.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", appLabel, obj.Name),
	})
	if err != nil {
		return obj, err
	}
	for _, revision := range revisions.Items {
		if err := appRevisionClient.Delete(revision.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return obj, err
		}
	}
	if !(obj.Annotations["cattle.io/skipUninstall"] == "true") {
		tempDirs, err := createTempDir(obj)
		if err != nil {
			return obj, err
		}
		defer os.RemoveAll(tempDirs.FullPath)
		tokenName, err := l.writeKubeConfig(obj, tempDirs.KubeConfigFull, true)
		if err != nil {
			return obj, err
		}
		defer func() {
			if err := l.systemTokens.DeleteToken(tokenName); err != nil {
				logrus.Errorf("cleanup for helm token [%s] failed, will not retry: %v", tokenName, err)
			}
		}()
		// try three times and succeed
		start := time.Second * 1
		for i := 0; i < 3; i++ {
			if err = helmDelete(tempDirs, obj); err == nil {
				break
			}
			logrus.Warn(err)
			time.Sleep(start)
			start *= 2
		}
	}
	ns, err := l.NsClient.Get(obj.Spec.TargetNamespace, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return obj, err
	} else if errors.IsNotFound(err) {
		return obj, nil
	}
	if val, ok := ns.Labels[MultiClusterAppIDSelector]; ok && val == obj.Spec.MultiClusterAppName {
		err := l.NsClient.Delete(ns.Name, &metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return obj, nil
		}
		return obj, err
	}
	appIds := strings.Split(ns.Annotations[AppIDsLabel], ",")
	appAnno := ""
	for _, appID := range appIds {
		if appID == obj.Name {
			continue
		}
		appAnno += appID + ","
	}
	if appAnno == "" {
		delete(ns.Annotations, AppIDsLabel)
	} else {
		appAnno = strings.TrimSuffix(appAnno, ",")
		ns.Annotations[AppIDsLabel] = appAnno
	}
	if _, err := l.NsClient.Update(ns); err != nil {
		return obj, err
	}
	return obj, nil
}

func (l *Lifecycle) Run(obj *v3.App, template string, tempDirs *hCommon.HelmPath) error {
	defer l.pruneOldRevisions(obj)
	tokenName, err := l.writeKubeConfig(obj, tempDirs.KubeConfigFull, false)
	if err != nil {
		return err
	}
	defer func() {
		if err := l.systemTokens.DeleteToken(tokenName); err != nil {
			logrus.Errorf("cleanup for helm token [%s] failed, will not retry: %v", tokenName, err)
		}
	}()
	notes, err := helmInstall(tempDirs, obj)
	if err != nil {
		// create an app revision so that user can decide to continue
		err2 := l.createAppRevision(obj, template, notes, true)
		if err2 != nil {
			return errorsutil.Wrapf(err, "error encountered while creating appRevision %v",
				err2)
		}
		return err
	}
	return l.createAppRevision(obj, template, notes, false)
}

// pruneOldRevisions checks if the total number of app revisions does not exceed the set maximum.
// If it does, the method deletes the oldest revision(s) to maintain the maximum number of revisions and no more.
func (l *Lifecycle) pruneOldRevisions(obj *v3.App) {
	if obj.Spec.MaxRevisionCount < 1 {
		obj.Spec.MaxRevisionCount = defaultMaxRevisionCount
	}
	_, projectName := ref.Parse(obj.Spec.ProjectName)
	appRevisionClient := l.AppRevisionGetter.AppRevisions(projectName)
	revisions, err := appRevisionClient.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", appLabel, obj.Name),
	})
	if err != nil {
		logrus.Warnf("[helm-controller] Failed to list revisions for app %s: %v", projectName, err)
		return
	}
	// Sort revisions by creation timestamp, starting with the oldest.
	sort.Slice(revisions.Items, func(i, j int) bool {
		return revisions.Items[i].CreationTimestamp.Before(&revisions.Items[j].CreationTimestamp)
	})
	if len(revisions.Items) > obj.Spec.MaxRevisionCount {
		logrus.Tracef("[helm-controller] App %s is exceeding the maximum (%d) number of stored revisions", obj.Name, obj.Spec.MaxRevisionCount)
		deleteCount := len(revisions.Items) - obj.Spec.MaxRevisionCount
		for _, revision := range revisions.Items[:deleteCount] {
			if err := appRevisionClient.Delete(revision.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				logrus.Warnf("[helm-controller] Failed to delete app revision %s: %v", revision.Name, err)
			}
		}
	}
}

func (l *Lifecycle) createAppRevision(obj *v3.App, template, notes string, failed bool) error {
	_, projectName := ref.Parse(obj.Spec.ProjectName)
	appRevisionClient := l.AppRevisionGetter.AppRevisions(projectName)
	release := &v3.AppRevision{}
	release.GenerateName = "apprevision-"
	release.Labels = map[string]string{
		appLabel: obj.Name,
	}
	if failed {
		release.Labels[failedLabel] = "true"
	}
	release.Spec.ProjectName = obj.Spec.ProjectName
	release.Status.Answers = obj.Spec.Answers
	release.Status.AnswersSetString = obj.Spec.AnswersSetString
	release.Status.ExternalID = obj.Spec.ExternalID
	release.Status.ValuesYaml = obj.Spec.ValuesYaml
	release.Status.Files = obj.Spec.Files

	digest := sha256.New()
	digest.Write([]byte(template))
	tag := hex.EncodeToString(digest.Sum(nil))
	release.Status.Digest = tag

	createdRevision, err := appRevisionClient.Create(release)
	if err != nil {
		return err
	}
	obj.Spec.AppRevisionName = createdRevision.Name
	obj.Status.Notes = notes
	if obj.Spec.Files != nil {
		obj.Status.AppliedFiles = obj.Spec.Files
	}
	return err
}

func (l *Lifecycle) writeKubeConfig(obj *v3.App, kubePath string, remove bool) (string, error) {
	var token string

	userID := obj.Annotations["field.cattle.io/creatorId"]
	user, err := l.UserClient.Get(userID, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", err
	} else if errors.IsNotFound(err) && remove {
		token, err = l.SystemAccountManager.CreateProjectHelmSystemToken(obj.Namespace)
	} else if err == nil {
		token, err = l.systemTokens.EnsureSystemToken(helmTokenPrefix+user.Name, description, "helm", user.Name, nil, true)
	}
	if err != nil {
		return "", err
	}

	kubeConfig := l.KubeConfigGetter.KubeConfig(l.ClusterName, token)
	for k := range kubeConfig.Clusters {
		kubeConfig.Clusters[k].InsecureSkipTLSVerify = true
	}

	if err := clientcmd.WriteToFile(*kubeConfig, kubePath); err != nil {
		return "", err
	}
	tokenID, _ := tokens.SplitTokenParts(token)
	return tokenID, nil
}

func isSame(obj *v3.App, revision *v3.AppRevision) bool {
	if obj.Spec.ExternalID != "" {
		if revision.Status.ExternalID == obj.Spec.ExternalID &&
			reflect.DeepEqual(revision.Status.Answers, obj.Spec.Answers) &&
			reflect.DeepEqual(revision.Status.AnswersSetString, obj.Spec.AnswersSetString) &&
			reflect.DeepEqual(revision.Status.ValuesYaml, obj.Spec.ValuesYaml) {
			return true
		}
		return false
	}

	if obj.Status.AppliedFiles != nil {
		if reflect.DeepEqual(obj.Status.AppliedFiles, obj.Spec.Files) &&
			reflect.DeepEqual(revision.Status.Answers, obj.Spec.Answers) &&
			reflect.DeepEqual(revision.Status.AnswersSetString, obj.Spec.AnswersSetString) &&
			reflect.DeepEqual(revision.Status.ValuesYaml, obj.Spec.ValuesYaml) {
			return true
		}
		return false
	}

	return false
}

func (l *Lifecycle) getHelmVersion(obj *v3.App) (string, error) {
	templateVersionID, templateVersionNamespace, err := hCommon.ParseExternalID(obj.Spec.ExternalID)
	if err != nil {
		return "", err
	}
	templateVersion, err := l.TemplateVersionLister.Get(templateVersionNamespace, templateVersionID)
	if err != nil {
		return "", err
	}
	return templateVersion.Status.HelmVersion, nil
}
