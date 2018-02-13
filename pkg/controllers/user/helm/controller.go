package helm

import (
	"context"
	"io/ioutil"

	"os"
	"path/filepath"

	"fmt"
	"time"

	"strings"

	"net/url"

	"github.com/rancher/norman/types/slice"
	hutils "github.com/rancher/rancher/pkg/controllers/user/helm/utils"
	core "github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	cacheRoot = "helm-controller"
)

func Register(user *config.UserContext) {
	stackClient := user.Management.Project.Apps("")
	stackLifecycle := &Lifecycle{
		NameSpaceClient:       user.Core.Namespaces(""),
		K8sClient:             user.K8sClient,
		TemplateVersionClient: user.Management.Management.TemplateVersions(""),
		CacheRoot:             filepath.Join(os.Getenv("HOME"), cacheRoot),
		Management:            user,
	}
	stackClient.AddLifecycle("helm-controller", stackLifecycle)
}

type Lifecycle struct {
	Management            *config.UserContext
	NameSpaceClient       core.NamespaceInterface
	TemplateVersionClient mgmtv3.TemplateVersionInterface
	K8sClient             kubernetes.Interface
	CacheRoot             string
}

func (l *Lifecycle) Create(obj *v3.App) (*v3.App, error) {
	if !l.isCurrentProject(obj) {
		return obj, nil
	}
	externalID := obj.Spec.ExternalID
	if externalID != "" {
		templateVersionID, err := parseExternalID(externalID)
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
	}
	return obj, nil
}

func (l *Lifecycle) Updated(obj *v3.App) (*v3.App, error) {
	if !l.isCurrentProject(obj) {
		return obj, nil
	}
	externalID := obj.Spec.ExternalID
	if externalID != "" {
		templateVersionID, err := parseExternalID(externalID)
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
	return obj, nil
}

func (l *Lifecycle) Remove(obj *v3.App) (*v3.App, error) {
	if !l.isCurrentProject(obj) {
		return obj, nil
	}
	externalID := obj.Spec.ExternalID
	if externalID != "" {
		templateVersionID, err := parseExternalID(externalID)
		if err != nil {
			return obj, err
		}
		if err := l.Run(obj, "delete", templateVersionID); err != nil {
			return obj, err
		}
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
	dir, err := l.writeTempFolder(templateVersion)
	if err != nil {
		return err
	}
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := generateRandomPort()
	probeAddr := generateRandomPort()
	data, err := yaml.Marshal(hutils.RestToRaw(l.Management.RESTConfig))
	if err != nil {
		return err
	}
	// todo: remove
	fmt.Println(string(data))
	if err := os.MkdirAll(filepath.Join(l.CacheRoot, obj.Namespace), 0755); err != nil {
		return err
	}
	kubeConfigPath := filepath.Join(l.CacheRoot, obj.Namespace, ".kubeconfig")
	if err := ioutil.WriteFile(kubeConfigPath, data, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(kubeConfigPath)
	go startTiller(cont, addr, probeAddr, obj.Spec.InstallNamespace, kubeConfigPath, obj.Spec.User, obj.Spec.Groups)
	switch action {
	case "install":
		if err := installCharts(dir, addr, obj); err != nil {
			return err
		}
	case "delete":
		if err := deleteCharts(addr, obj); err != nil {
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

func (l *Lifecycle) isCurrentProject(obj *v3.App) bool {
	projectID := obj.Spec.ProjectName
	clusterName := strings.Split(projectID, ":")[0]
	if clusterName == l.Management.ClusterName {
		return true
	}
	return false
}

func parseExternalID(externalID string) (string, error) {
	values, err := url.ParseQuery(externalID)
	if err != nil {
		return "", err
	}
	catalog := values.Get("catalog://?catalog")
	base := values.Get("base")
	template := values.Get("template")
	version := values.Get("version")
	return strings.Join([]string{catalog, base, template, version}, "-"), nil
}
