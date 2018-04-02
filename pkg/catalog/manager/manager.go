package manager

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/git"
	"github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HelmTemplateBaseType = "kubernetes"
)

type Manager struct {
	cacheRoot             string
	httpClient            http.Client
	uuid                  string
	catalogClient         v3.CatalogInterface
	templateClient        v3.TemplateInterface
	templateVersionClient v3.TemplateVersionInterface
	templateContentClient v3.TemplateContentInterface
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
		templateContentClient: management.Management.TemplateContents(""),
	}
}

func (m *Manager) GetCatalogs() ([]v3.Catalog, error) {
	list, err := m.catalogClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (m *Manager) prepareRepoPath(catalog v3.Catalog) (path string, commit string, err error) {
	if git.IsValid(catalog.Spec.URL) {
		path, commit, err = m.prepareGitRepoPath(catalog)
	} else {
		path, commit, err = m.prepareHelmRepoPath(catalog)
	}
	return
}

func (m *Manager) prepareHelmRepoPath(catalog v3.Catalog) (string, string, error) {
	index, err := helm.DownloadIndex(catalog.Spec.URL)
	if err != nil {
		return "", "", err
	}

	repoPath := path.Join(m.cacheRoot, catalog.Namespace, index.Hash)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", "", err
	}

	if err := helm.SaveIndex(index, repoPath); err != nil {
		return "", "", err
	}

	return repoPath, index.Hash, nil
}

func hash(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) prepareGitRepoPath(catalog v3.Catalog) (string, string, error) {
	branch := catalog.Spec.Branch
	if branch == "" {
		branch = "master"
	}

	repoBranchHash := hash(catalog.Spec.URL + branch)
	repoPath := path.Join(m.cacheRoot, repoBranchHash)

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", "", err
	}

	empty, err := dirEmpty(repoPath)
	if err != nil {
		return "", "", errors.Wrap(err, "Empty directory check failed")
	}

	if empty {
		if err = git.Clone(repoPath, catalog.Spec.URL, branch); err != nil {
			return "", "", errors.Wrap(err, "Clone failed")
		}
	} else {
		changed, err := m.remoteShaChanged(catalog.Spec.URL, catalog.Spec.Branch, catalog.Status.Commit, m.uuid)
		if err != nil {
			return "", "", errors.Wrap(err, "Remote commit check failed")
		}
		if changed {
			if err = git.Update(repoPath, branch); err != nil {
				return "", "", errors.Wrap(err, "Update failed")
			}
			logrus.Debugf("catalog-service: updated catalog '%v'", catalog.Name)
		}
	}

	commit, err := git.HeadCommit(repoPath)
	if err != nil {
		err = errors.Wrap(err, "Retrieving head commit failed")
	}
	return repoPath, commit, err
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
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
