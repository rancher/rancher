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
	"github.com/rancher/rancher/pkg/auth/providers/local"
	hutils "github.com/rancher/rancher/pkg/controllers/user/helm/utils"
	"github.com/rancher/rancher/pkg/randomtoken"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	helmTokenPrefix = "helm-token-"
)

func Register(user *config.UserContext) {
	appClient := user.Management.Project.Apps("")
	stackLifecycle := &Lifecycle{
		TokenClient:           user.Management.Management.Tokens(""),
		UserClient:            user.Management.Management.Users(""),
		K8sClient:             user.K8sClient,
		TemplateVersionClient: user.Management.Management.TemplateVersions(""),
		ListenConfigClient:    user.Management.Management.ListenConfigs(""),
		ClusterName:           user.ClusterName,
	}
	appClient.AddClusterScopedLifecycle("helm-controller", user.ClusterName, stackLifecycle)
}

type Lifecycle struct {
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
	token := ""
	if t, err := l.TokenClient.Get(helmTokenPrefix+user.Name, metav1.GetOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		token, err = l.generateToken(user)
		if err != nil {
			return err
		}
	} else {
		token = t.Name + ":" + t.Token
	}

	data, err := yaml.Marshal(hutils.RestToRaw(token, l.ClusterName))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(tempDir, obj.Namespace), 0755); err != nil {
		return err
	}
	kubeConfigPath := filepath.Join(tempDir, obj.Namespace, ".kubeconfig")
	if err := ioutil.WriteFile(kubeConfigPath, data, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(kubeConfigPath)
	go hutils.StartTiller(cont, addr, probeAddr, obj.Spec.InstallNamespace, kubeConfigPath, obj.Spec.User, obj.Spec.Groups)
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

const userIDLabel = "authn.management.cattle.io/token-userId"

// this might be pretty hacky, need to revisit
func (l *Lifecycle) generateToken(user *mgmtv3.User) (string, error) {
	token := mgmtv3.Token{
		TTLMillis:    0,
		Description:  "token for helm chart deployment",
		UserID:       user.Name,
		AuthProvider: local.Name,
		IsDerived:    false,
	}
	key, err := randomtoken.Generate()
	if err != nil {
		return "", fmt.Errorf("failed to generate token key")
	}

	labels := make(map[string]string)
	labels[userIDLabel] = token.UserID

	token.APIVersion = "management.cattle.io/v3"
	token.Kind = "Token"
	token.Token = key
	token.ObjectMeta = metav1.ObjectMeta{
		Name:   helmTokenPrefix + user.Name,
		Labels: labels,
	}
	createdToken, err := l.TokenClient.Create(&token)

	if err != nil {
		return "", err
	}
	return createdToken.Name + ":" + createdToken.Token, nil
}

func (l *Lifecycle) saveTemplates(obj *v3.App, templateVersion *mgmtv3.TemplateVersion) error {
	templates := map[string]string{}
	for _, file := range templateVersion.Spec.Files {
		templates[file.Name] = file.Contents
	}
	obj.Spec.Templates = templates
	return nil
}
