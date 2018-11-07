package helm

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/git"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func New(catalog *v3.Catalog) (*Helm, error) {
	h, err := NewNoUpdate(catalog)
	if err != nil {
		return nil, err
	}
	return h, h.UpdateIfCacheNotExists()
}

func NewNoUpdate(catalog *v3.Catalog) (*Helm, error) {
	if catalog == nil || catalog.Name == "" {
		return nil, errors.New("Catalog is not defined for helm")
	}
	return newCache(catalog), nil
}

func NewForceUpdate(catalog *v3.Catalog) (string, *Helm, error) {
	h, err := NewNoUpdate(catalog)
	if err != nil {
		return "", nil, err
	}
	commit, err := h.Update()
	return commit, h, err
}

func CatalogSHA256Hash(catalog *v3.Catalog) string {
	url := catalog.Spec.URL
	branch := catalog.Spec.Branch
	username := catalog.Spec.Username
	password := catalog.Spec.Password
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s %s %s %s", url, branch, username, password)))
	return hex.EncodeToString(hashBytes[:])
}

func newCache(catalog *v3.Catalog) *Helm {
	hash := CatalogSHA256Hash(catalog)
	return &Helm{
		LocalPath:   filepath.Join(CatalogCache, hash),
		IconPath:    filepath.Join(IconCache, hash),
		catalogName: catalog.Name,
		Hash:        hash,
		url:         catalog.Spec.URL,
		branch:      catalog.Spec.Branch,
		username:    catalog.Spec.Username,
		password:    catalog.Spec.Password,
		lastCommit:  catalog.Status.Commit,
	}
}

func (h *Helm) UpdateIfCacheNotExists() error {
	h.lock()
	defer h.unlock()

	if _, err := os.Stat(h.LocalPath); os.IsNotExist(err) {
		if _, err = h.update(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(h.IconPath, 0755); err != nil {
		return err
	}
	return nil
}

func (h *Helm) Update() (string, error) {
	h.lock()
	defer h.unlock()

	commit, err := h.update()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(h.IconPath, 0755); err != nil {
		return "", err
	}
	return commit, nil
}

func (h *Helm) update() (string, error) {
	logrus.Debugf("Helm preparing catalog cache [%s] for update", h.catalogName)
	if err := os.MkdirAll(h.LocalPath, 0755); err != nil {
		return "", err
	}

	pathURL := git.FormatURL(h.url, h.username, h.password)
	var (
		commit string
		err    error
	)
	if git.IsValid(pathURL) {
		commit, err = h.updateGit(pathURL)
	} else {
		commit, err = h.updateIndex()
	}
	return commit, err
}

func (h *Helm) updateIndex() (string, error) {
	index, err := h.downloadIndex(h.url)
	if err != nil {
		return "", err
	}

	if err := h.saveIndex(index); err != nil {
		return "", err
	}
	logrus.Debugf("Helm updated http-helm catalog [%s]", h.url)
	return index.Hash, nil
}

func md5Hash(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (h *Helm) updateGit(repoURL string) (string, error) {
	if h.branch == "" {
		h.branch = "master"
	}

	empty, err := dirEmpty(h.LocalPath)
	if err != nil {
		return "", errors.Wrap(err, "Empty directory check failed")
	}

	if empty {
		if err = git.Clone(h.LocalPath, repoURL, h.branch); err != nil {
			return "", errors.Wrap(err, "Clone failed")
		}
	} else {
		changed, err := remoteShaChanged(repoURL, h.branch, h.lastCommit, uuid)
		if err != nil {
			return "", errors.Wrap(err, "Remote commit check failed")
		}
		if changed {
			if err = git.Update(h.LocalPath, h.branch); err != nil {
				return "", errors.Wrap(err, "Update failed")
			}
			logrus.Debugf("Helm updated git repository for catalog [%s]", h.catalogName)
		}
	}

	commit, err := git.HeadCommit(h.LocalPath)
	if err != nil {
		err = errors.Wrap(err, "Retrieving head commit failed")
	}

	logrus.Debugf("Helm updated git-helm catalog [%s:%s]", repoURL, h.branch)
	return commit, err
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

func remoteShaChanged(repoURL, branch, sha, uuid string) (bool, error) {
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
	resp, err := httpClient.Do(req)
	if err != nil {
		// Return timeout errors so caller can decide whether or not to proceed with updating the repo
		if uErr, ok := err.(*url.Error); ok && uErr.Timeout() {
			return false, errors.Wrapf(uErr, "Repo [%v] is not accessible", repoURL)
		}
		return true, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
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
