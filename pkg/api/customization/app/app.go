package app

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"encoding/base64"
	"net/url"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	hutils "github.com/rancher/rancher/pkg/controllers/user/helm/utils"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	managementv3 "github.com/rancher/types/client/management/v3"
	projectv3 "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	base       = 32768
	end        = 61000
	tillerName = "tiller"
	helmName   = "helm"
	cacheRoot  = "helm-controller"
)

type ActionWrapper struct {
	Management config.ManagementContext
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "upgrade")
	resource.AddAction(apiContext, "rollback")
}

func (a ActionWrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var stack projectv3.App
	if err := access.ByID(apiContext, &projectschema.Version, projectv3.AppType, apiContext.ID, &stack); err != nil {
		return err
	}
	clusterName := strings.Split(stack.ProjectId, ":")[0]
	var cluster managementv3.Cluster
	if err := access.ByID(apiContext, &managementschema.Version, managementv3.ClusterType, clusterName, &cluster); err != nil {
		return err
	}
	clusterRestConfig, err := a.toRESTConfig(&cluster)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(hutils.RestToRaw(*clusterRestConfig))
	if err != nil {
		return err
	}
	// todo: remove
	fmt.Println(string(data))
	rootDir := filepath.Join(os.Getenv("HOME"), cacheRoot)
	if err := os.MkdirAll(filepath.Join(rootDir, stack.Name), 0755); err != nil {
		return err
	}
	kubeConfigPath := filepath.Join(rootDir, stack.Name, ".kubeconfig")
	if err := ioutil.WriteFile(kubeConfigPath, data, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(kubeConfigPath)
	store := apiContext.Schema.Store
	switch actionName {
	case "upgrade":
		cont, cancel := context.WithCancel(context.Background())
		defer cancel()
		addr := generateRandomPort()
		probeAddr := generateRandomPort()
		go startTiller(cont, addr, probeAddr, stack.InstallNamespace, kubeConfigPath, stack.User, stack.Groups)
		actionInput, err := parse.ReadBody(apiContext.Request)
		if err != nil {
			return err
		}
		externalID := actionInput["externalId"]
		updateData := map[string]interface{}{}
		updateData["externalId"] = externalID
		_, err = store.Update(apiContext, apiContext.Schema, updateData, apiContext.ID)
		if err != nil {
			return err
		}
		templateVersionID, err := parseExternalID(convert.ToString(externalID))
		if err != nil {
			return err
		}
		var templateVersion managementv3.TemplateVersion
		if err := access.ByID(apiContext, &managementschema.Version, managementv3.TemplateVersionType, templateVersionID, &templateVersion); err != nil {
			return err
		}
		files := convertTemplates(templateVersion.Files)
		rootDir := filepath.Join(os.Getenv("HOME"), cacheRoot)
		tempDir, err := writeTempDir(rootDir, files)
		if err != nil {
			return err
		}
		if err := upgradeCharts(tempDir, addr, stack.Name); err != nil {
			return err
		}
		_, err = store.Update(apiContext, apiContext.Schema, updateData, apiContext.ID)
		if err != nil {
			return err
		}
		return nil
	case "rollback":
		cont, cancel := context.WithCancel(context.Background())
		defer cancel()
		addr := generateRandomPort()
		probeAddr := generateRandomPort()
		go startTiller(cont, addr, probeAddr, stack.InstallNamespace, kubeConfigPath, stack.User, stack.Groups)
		actionInput, err := parse.ReadBody(apiContext.Request)
		if err != nil {
			return err
		}
		revision := actionInput["revision"]
		if err := rollbackCharts(addr, stack.Name, convert.ToString(revision)); err != nil {
			return err
		}
		data := map[string]interface{}{
			"name": apiContext.ID,
		}
		_, err = store.Update(apiContext, apiContext.Schema, data, apiContext.ID)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func writeTempDir(rootDir string, files map[string]string) (string, error) {
	for name, content := range files {
		fp := filepath.Join(rootDir, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return "", err
		}
		if err := ioutil.WriteFile(fp, []byte(content), 0755); err != nil {
			return "", err
		}
	}
	for name := range files {
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			return filepath.Join(rootDir, parts[0]), nil
		}
	}
	return "", nil
}

func convertTemplates(files []managementv3.File) map[string]string {
	templates := map[string]string{}
	for _, f := range files {
		templates[f.Name] = f.Contents
	}
	return templates
}

func upgradeCharts(rootDir, port, releaseName string) error {
	cmd := exec.Command(helmName, "upgrade", "--namespace", releaseName, releaseName, rootDir)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func rollbackCharts(port, releaseName, revision string) error {
	cmd := exec.Command(helmName, "rollback", releaseName, revision)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func generateRandomPort() string {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	port := base + r1.Intn(end-base+1)
	return strconv.Itoa(port)
}

// startTiller start tiller server and return the listening address of the grpc address
func startTiller(context context.Context, port, probePort, namespace, kubeConfigPath, user string, groups []string) error {
	groupsAsString := strings.Join(groups, ",")
	cmd := exec.Command(tillerName, "--listen", ":"+port, "--probe", ":"+probePort, "--user", user, "--groups", groupsAsString)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "KUBECONFIG", kubeConfigPath), fmt.Sprintf("%s=%s", "TILLER_NAMESPACE", namespace)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	select {
	case <-context.Done():
		return cmd.Process.Kill()
	}
}

func (a ActionWrapper) toRESTConfig(cluster *client.Cluster) (*rest.Config, error) {
	cls, err := a.Management.Management.Clusters("").Get(cluster.ID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if cls == nil {
		return nil, nil
	}

	if cluster.Internal {
		return a.Management.LocalConfig, nil
	}

	if cluster.APIEndpoint == "" || cluster.CACert == "" || cls.Status.ServiceAccountToken == "" {
		return nil, nil
	}

	u, err := url.Parse(cluster.APIEndpoint)
	if err != nil {
		return nil, err
	}

	data, err := base64.StdEncoding.DecodeString(cluster.CACert)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: cls.Status.ServiceAccountToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: data,
		},
	}, nil
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
