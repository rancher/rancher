package manager

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path"

	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/catalog-controller/git"
	"github.com/rancher/catalog-controller/helm"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HelmTemplateType     = "helm"
	RancherTemplateType  = "native"
	HelmTemplateBaseType = "kubernetes"
	ProjectLabel         = "io.cattle.catalog.project_id"
)

type CatalogType int

const (
	CatalogTypeRancher CatalogType = iota
	CatalogTypeHelmObjectRepo
	CatalogTypeHelmGitRepo
	CatalogTypeInvalid
)

type Manager struct {
	cacheRoot             string
	httpClient            http.Client
	uuid                  string
	catalogClient         v3.CatalogInterface
	templateClient        v3.TemplateInterface
	templateVersionClient v3.TemplateVersionInterface
	lastUpdateTime        time.Time
}

func New(management *config.ManagementContext, cacheRoot string) *Manager {
	// todo: figure out uuid
	uuid := "9bf84dcd-8011-4f21-a24e-fc0c979026a3"
	return &Manager{
		cacheRoot: cacheRoot,
		httpClient: http.Client{
			Timeout: time.Second * 10,
		},
		uuid:                  uuid,
		catalogClient:         management.Management.Catalogs(""),
		templateClient:        management.Management.Templates(""),
		templateVersionClient: management.Management.TemplateVersions(""),
	}
}

func (m *Manager) GetCatalogs() ([]v3.Catalog, error) {
	list, err := m.catalogClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (m *Manager) prepareRepoPath(catalog v3.Catalog, update bool) (string, string, CatalogType, error) {
	if catalog.Spec.CatalogKind == "" || catalog.Spec.CatalogKind == RancherTemplateType {
		return m.prepareGitRepoPath(catalog, update, CatalogTypeRancher)
	}
	if catalog.Spec.CatalogKind == HelmTemplateType {
		if git.IsValid(catalog.Spec.URL) {
			return m.prepareGitRepoPath(catalog, update, CatalogTypeHelmGitRepo)
		}
		return m.prepareHelmRepoPath(catalog, update)
	}
	return "", "", CatalogTypeInvalid, fmt.Errorf("Unknown catalog kind=%s", catalog.Kind)
}

func (m *Manager) prepareHelmRepoPath(catalog v3.Catalog, update bool) (string, string, CatalogType, error) {
	index, err := helm.DownloadIndex(catalog.Spec.URL)
	if err != nil {
		return "", "", CatalogTypeInvalid, err
	}

	repoPath := path.Join(m.cacheRoot, catalog.Labels[ProjectLabel], index.Hash)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", "", CatalogTypeInvalid, err
	}

	if err := helm.SaveIndex(index, repoPath); err != nil {
		return "", "", CatalogTypeInvalid, err
	}

	return repoPath, index.Hash, CatalogTypeHelmObjectRepo, nil
}

func (m *Manager) prepareGitRepoPath(catalog v3.Catalog, update bool, catalogType CatalogType) (string, string, CatalogType, error) {
	branch := catalog.Spec.Branch
	if catalog.Spec.Branch == "" {
		branch = "master"
	}

	sum := md5.Sum([]byte(catalog.Spec.URL + branch))
	repoBranchHash := hex.EncodeToString(sum[:])
	repoPath := path.Join(m.cacheRoot, catalog.Labels[ProjectLabel], repoBranchHash)

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", "", catalogType, errors.Wrap(err, "mkdir failed")
	}

	empty, err := dirEmpty(repoPath)
	if err != nil {
		return "", "", catalogType, errors.Wrap(err, "Empty directory check failed")
	}

	if empty {
		if err = git.Clone(repoPath, catalog.Spec.URL, branch); err != nil {
			return "", "", catalogType, errors.Wrap(err, "Clone failed")
		}
	} else {
		if update {
			changed, err := m.remoteShaChanged(catalog.Spec.URL, catalog.Spec.Branch, catalog.Status.Commit, m.uuid)
			if err != nil {
				return "", "", catalogType, errors.Wrap(err, "Remote commit check failed")
			}
			if changed {
				if err = git.Update(repoPath, branch); err != nil {
					return "", "", catalogType, errors.Wrap(err, "Update failed")
				}
				logrus.Debugf("catalog-service: updated catalog '%v'", catalog.Name)
			}
		}
	}

	commit, err := git.HeadCommit(repoPath)
	if err != nil {
		err = errors.Wrap(err, "Retrieving head commit failed")
	}
	return repoPath, commit, catalogType, err
}

func formatGitURL(endpoint, branch string) string {
	formattedURL := ""
	if u, err := url.Parse(endpoint); err == nil {
		pathParts := strings.Split(u.Path, "/")
		switch strings.Split(u.Host, ":")[0] {
		case "github.com":
			if len(pathParts) >= 3 {
				org := pathParts[1]
				repo := strings.TrimSuffix(pathParts[2], ".git")
				formattedURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", org, repo, branch)
			}
		case "git.rancher.io":
			repo := strings.TrimSuffix(pathParts[1], ".git")
			u.Path = fmt.Sprintf("/repos/%s/commits/%s", repo, branch)
			formattedURL = u.String()
		}
	}
	return formattedURL
}

func (m *Manager) remoteShaChanged(repoURL, branch, sha, uuid string) (bool, error) {
	formattedURL := formatGitURL(repoURL, branch)

	if formattedURL == "" {
		return true, nil
	}

	req, err := http.NewRequest("GET", formattedURL, nil)
	if err != nil {
		logrus.Warnf("Problem creating request to check git remote sha of repo [%v]: %v", repoURL, err)
		return true, nil
	}
	req.Header.Set("Accept", "application/vnd.github.chitauri-preview+sha")
	req.Header.Set("If-None-Match", fmt.Sprintf("\"%s\"", sha))
	if uuid != "" {
		req.Header.Set("X-Install-Uuid", uuid)
	}
	res, err := m.httpClient.Do(req)
	if err != nil {
		// Return timeout errors so caller can decide whether or not to proceed with updating the repo
		if uErr, ok := err.(*url.Error); ok && uErr.Timeout() {
			return false, errors.Wrapf(uErr, "Repo [%v] is not accessible", repoURL)
		}
		return true, nil
	}
	defer res.Body.Close()

	if res.StatusCode == 304 {
		return false, nil
	}

	return true, nil
}

func dirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
