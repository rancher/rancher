package helm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	"github.com/rancher/rancher/pkg/ref"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	helmTokenPrefix = "helm-token-"
	description     = "token for helm chart deployment"
)

func Register(user *config.UserContext, kubeConfigGetter common.KubeConfigGetter) {
	appClient := user.Management.Project.Apps("")
	stackLifecycle := &Lifecycle{
		KubeConfigGetter:      kubeConfigGetter,
		TokenClient:           user.Management.Management.Tokens(""),
		UserClient:            user.Management.Management.Users(""),
		UserManager:           user.Management.UserManager,
		K8sClient:             user.K8sClient,
		TemplateVersionClient: user.Management.Management.TemplateVersions(""),
		ListenConfigClient:    user.Management.Management.ListenConfigs(""),
		ClusterName:           user.ClusterName,
		TemplateContentClient: user.Management.Management.TemplateContents(""),
		AppRevisionGetter:     user.Management.Project,
		AppGetter:             user.Management.Project,
	}
	appClient.AddClusterScopedLifecycle("helm-controller", user.ClusterName, stackLifecycle)
}

type Lifecycle struct {
	KubeConfigGetter      common.KubeConfigGetter
	UserManager           user.Manager
	TokenClient           mgmtv3.TokenInterface
	UserClient            mgmtv3.UserInterface
	TemplateVersionClient mgmtv3.TemplateVersionInterface
	K8sClient             kubernetes.Interface
	ListenConfigClient    mgmtv3.ListenConfigInterface
	ClusterName           string
	TemplateContentClient mgmtv3.TemplateContentInterface
	AppRevisionGetter     v3.AppRevisionsGetter
	AppGetter             v3.AppsGetter
}

func (l *Lifecycle) Create(obj *v3.App) (*v3.App, error) {
	v3.AppConditionMigrated.True(obj)
	return obj, nil
}

func (l *Lifecycle) Updated(obj *v3.App) (*v3.App, error) {
	if obj.Spec.ExternalID == "" && len(obj.Spec.Files) == 0 {
		return obj, nil
	}
	// always refresh app to avoid updating app twice
	_, projectName := ref.Parse(obj.Spec.ProjectName)
	var err error
	obj, err = l.AppGetter.Apps(projectName).Get(obj.Name, metav1.GetOptions{})
	if err != nil {
		return obj, err
	}
	// if app was created before 2.1, run migrate to install a no-op helm release
	newObj, err := v3.AppConditionMigrated.Once(obj, func() (runtime.Object, error) {
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
		if obj.Spec.ExternalID != "" {
			if currentRevision.Status.ExternalID == obj.Spec.ExternalID && reflect.DeepEqual(currentRevision.Status.Answers, obj.Spec.Answers) {
				return obj, nil
			}
		}
		if obj.Status.AppliedFiles != nil {
			if reflect.DeepEqual(obj.Status.AppliedFiles, obj.Spec.Files) && reflect.DeepEqual(currentRevision.Status.Answers, obj.Spec.Answers) {
				return obj, nil
			}
		}
	}
	return l.DeployApp(obj)
}

func (l *Lifecycle) DeployApp(obj *v3.App) (*v3.App, error) {
	newObj, err := v3.AppConditionInstalled.Do(obj, func() (runtime.Object, error) {
		template, notes, tempDir, err := generateTemplates(obj, l.TemplateVersionClient, l.TemplateContentClient)
		defer os.RemoveAll(tempDir)
		if err != nil {
			return obj, err
		}
		if err := l.Run(obj, template, tempDir, notes); err != nil {
			return obj, err
		}
		return obj, nil
	})
	return newObj.(*v3.App), err
}

func (l *Lifecycle) Remove(obj *v3.App) (*v3.App, error) {
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
	tempDir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		return obj, err
	}
	defer os.RemoveAll(tempDir)
	kubeConfigPath, err := l.writeKubeConfig(obj, tempDir)
	if err != nil {
		return obj, err
	}
	// try three times and succeed
	start := time.Second * 1
	for i := 0; i < 3; i++ {
		if err := helmDelete(kubeConfigPath, obj); err == nil {
			break
		}
		logrus.Error(err)
		time.Sleep(start)
		start *= 2
	}
	return obj, nil
}

func (l *Lifecycle) Run(obj *v3.App, template, templateDir, notes string) error {
	tempDir, err := ioutil.TempDir("", "kubeconfig-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	kubeConfigPath, err := l.writeKubeConfig(obj, tempDir)
	if err != nil {
		return err
	}
	if err := helmInstall(templateDir, kubeConfigPath, obj); err != nil {
		return err
	}
	return l.createAppRevision(obj, template, notes, false)
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
	release.Status.Answers = obj.Spec.Answers
	release.Status.ProjectName = projectName
	release.Status.ExternalID = obj.Spec.ExternalID
	digest := sha256.New()
	digest.Write([]byte(template))
	tag := hex.EncodeToString(digest.Sum(nil))
	if _, err := l.TemplateContentClient.Get(tag, metav1.GetOptions{}); errors.IsNotFound(err) {
		tc := &mgmtv3.TemplateContent{}
		tc.Name = tag
		tc.Data = template
		if _, err := l.TemplateContentClient.Create(tc); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
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

func (l *Lifecycle) writeKubeConfig(obj *v3.App, tempDir string) (string, error) {
	userID := obj.Annotations["field.cattle.io/creatorId"]
	user, err := l.UserClient.Get(userID, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	token, err := l.UserManager.EnsureToken(helmTokenPrefix+user.Name, description, user.Name)
	if err != nil {
		return "", err
	}

	kubeConfig := l.KubeConfigGetter.KubeConfig(l.ClusterName, token)
	for k := range kubeConfig.Clusters {
		kubeConfig.Clusters[k].InsecureSkipTLSVerify = true
	}
	if err := os.MkdirAll(filepath.Join(tempDir, obj.Namespace), 0755); err != nil {
		return "", err
	}
	kubeConfigPath := filepath.Join(tempDir, obj.Namespace, ".kubeconfig")
	if err := clientcmd.WriteToFile(*kubeConfig, kubeConfigPath); err != nil {
		return "", err
	}
	return kubeConfigPath, nil
}
