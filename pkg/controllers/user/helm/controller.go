package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	hutils "github.com/rancher/rancher/pkg/controllers/user/helm/utils"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
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
}

func (l *Lifecycle) Create(obj *v3.App) (*v3.App, error) {
	if obj.Spec.ExternalID == "" {
		return obj, nil
	}
	newObj, err := v3.AppConditionInstalled.DoUntilTrue(obj, func() (runtime.Object, error) {
		templateVersionID, err := hutils.ParseExternalID(obj.Spec.ExternalID)
		if err != nil {
			return obj, err
		}
		if err := l.Run(obj, "install", templateVersionID); err != nil {
			return obj, err
		}
		configMaps, err := l.K8sClient.CoreV1().ConfigMaps(obj.Spec.InstallNamespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "NAME", obj.Name),
		})
		if err != nil {
			return obj, err
		}
		releases := []v3.ReleaseInfo{}
		for _, cm := range configMaps.Items {
			releaseInfo := v3.ReleaseInfo{}
			releaseInfo.Name = cm.Name
			releaseInfo.Version = cm.Labels["VERSION"]
			releaseInfo.CreateTimestamp = cm.CreationTimestamp.Format(time.RFC3339)
			releaseInfo.ModifiedAt = cm.Labels["MODIFIED_AT"]
			releaseInfo.TemplateVersionID = templateVersionID
			releases = append(releases, releaseInfo)
		}
		obj.Status.Releases = releases
		return obj, nil
	})
	return newObj.(*v3.App), err
}

func (l *Lifecycle) Updated(obj *v3.App) (*v3.App, error) {
	if obj.Spec.ExternalID == "" {
		return obj, nil
	}
	templateVersionID, err := hutils.ParseExternalID(obj.Spec.ExternalID)
	if err != nil {
		return obj, err
	}
	templateVersion, err := l.TemplateVersionClient.Get(templateVersionID, metav1.GetOptions{})
	if err != nil {
		return obj, err
	}
	if err := l.saveTemplates(obj, templateVersion); err != nil {
		return obj, err
	}
	configMaps, err := l.K8sClient.CoreV1().ConfigMaps(obj.Spec.InstallNamespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "NAME", obj.Name),
	})
	if err != nil {
		return obj, err
	}
	releases := obj.Status.Releases
	alreadyExistedReleaseNames := []string{}
	for _, k := range releases {
		alreadyExistedReleaseNames = append(alreadyExistedReleaseNames, k.Name)
	}
	for _, cm := range configMaps.Items {
		if !slice.ContainsString(alreadyExistedReleaseNames, cm.Name) {
			logrus.Infof("uploading release %s into namespace %s", cm.Name, obj.Name)
			releaseInfo := v3.ReleaseInfo{}
			releaseInfo.Name = cm.Name
			releaseInfo.Version = cm.Labels["VERSION"]
			releaseInfo.CreateTimestamp = cm.CreationTimestamp.Format(time.RFC3339)
			releaseInfo.ModifiedAt = cm.Labels["MODIFIED_AT"]
			releaseInfo.TemplateVersionID = templateVersionID
			releases = append(releases, releaseInfo)
		}
	}
	obj.Status.Releases = releases
	return obj, nil
}

func (l *Lifecycle) Remove(obj *v3.App) (*v3.App, error) {
	if obj.Spec.ExternalID == "" {
		return obj, nil
	}
	templateVersionID, err := hutils.ParseExternalID(obj.Spec.ExternalID)
	if err != nil {
		return obj, err
	}
	if err := l.Run(obj, "delete", templateVersionID); err != nil {
		return obj, err
	}
	return obj, nil
}

func (l *Lifecycle) Run(obj *v3.App, action, templateVersionID string) error {
	templateVersion, err := l.TemplateVersionClient.Get(templateVersionID, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if err := l.saveTemplates(obj, templateVersion); err != nil {
		return err
	}
	files := map[string]string{}
	for _, file := range templateVersion.Spec.Files {
		content, err := base64.StdEncoding.DecodeString(file.Contents)
		if err != nil {
			return err
		}
		files[file.Name] = string(content)
	}
	tempDir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		return err
	}
	dir, err := hutils.WriteTempDir(tempDir, files)
	defer os.RemoveAll(dir)
	if err != nil {
		return err
	}

	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := hutils.GenerateRandomPort()
	probeAddr := hutils.GenerateRandomPort()
	userID := obj.Annotations["field.cattle.io/creatorId"]
	user, err := l.UserClient.Get(userID, metav1.GetOptions{})
	if err != nil {
		return err
	}
	token, err := l.UserManager.EnsureToken(helmTokenPrefix+user.Name, description, user.Name)
	if err != nil {
		return err
	}

	kubeConfig := l.KubeConfigGetter.KubeConfig(l.ClusterName, token)
	if err := os.MkdirAll(filepath.Join(tempDir, obj.Namespace), 0755); err != nil {
		return err
	}
	kubeConfigPath := filepath.Join(tempDir, obj.Namespace, ".kubeconfig")
	if err := clientcmd.WriteToFile(*kubeConfig, kubeConfigPath); err != nil {
		return err
	}
	defer os.RemoveAll(kubeConfigPath)
	go hutils.StartTiller(cont, addr, probeAddr, obj.Spec.InstallNamespace, kubeConfigPath)
	switch action {
	case "install":
		if err := hutils.InstallCharts(dir, addr, obj); err != nil {
			return err
		}
	case "delete":
		if err := hutils.DeleteCharts(addr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lifecycle) saveTemplates(obj *v3.App, templateVersion *mgmtv3.TemplateVersion) error {
	templates := map[string]string{}
	for _, file := range templateVersion.Spec.Files {
		templates[file.Name] = file.Contents
	}
	obj.Spec.Templates = templates
	return nil
}
