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
	secretmigrator "github.com/rancher/rancher/pkg/controllers/management/secretmigrator/catalog"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/git"
	"github.com/sirupsen/logrus"
)

const (
	KindHelmGit      = "helm:git"
	KindHelmHTTP     = "helm:http"
	KindHelmInternal = "helm:internal"
)

var (
	validCatalogKind = map[string]bool{
		KindHelmGit:      true,
		KindHelmHTTP:     true,
		KindHelmInternal: true,
	}
)

func New(catalog *v3.Catalog, secretLister v1.SecretLister) (*Helm, error) {
	h, err := NewNoUpdate(catalog, secretLister)
	if err != nil {
		return nil, err
	}
	_, err = h.Update(false)
	return h, err
}

func NewNoUpdate(catalog *v3.Catalog, secretLister v1.SecretLister) (*Helm, error) {
	if catalog == nil || catalog.Name == "" {
		return nil, errors.New("Catalog is not defined for helm")
	}
	return newCache(catalog, secretLister)
}

func NewForceUpdate(catalog *v3.Catalog, secretLister v1.SecretLister) (string, *Helm, error) {
	h, err := NewNoUpdate(catalog, secretLister)
	if err != nil {
		return "", nil, err
	}
	commit, err := h.Update(true)
	return commit, h, err
}

func CatalogSHA256Hash(catalog *v3.Catalog) string {
	u := catalog.Spec.URL
	branch := catalog.Spec.Branch
	username := catalog.Spec.Username
	password := catalog.Spec.Password
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s %s %s %s", u, branch, username, password)))
	return hex.EncodeToString(hashBytes[:])
}

func newCache(catalog *v3.Catalog, secretLister v1.SecretLister) (*Helm, error) {
	catalog = catalog.DeepCopy()
	var err error
	catalog.Spec, err = secretmigrator.AssembleCatalogCredential(catalog, secretLister)
	if err != nil {
		return nil, err
	}
	hash := CatalogSHA256Hash(catalog)
	localPath := filepath.Join(CatalogCache, hash)
	kind := getCatalogKind(catalog, localPath)

	if kind == KindHelmInternal {
		localPath = filepath.Join(InternalCatalog, catalog.Name)
	}

	return &Helm{
		LocalPath:   localPath,
		IconPath:    filepath.Join(IconCache, hash),
		catalogName: catalog.Name,
		Hash:        hash,
		Kind:        kind,
		url:         catalog.Spec.URL,
		branch:      catalog.Spec.Branch,
		username:    catalog.Spec.Username,
		password:    catalog.Spec.Password,
		lastCommit:  catalog.Status.Commit,
	}, nil
}

func getCatalogKind(catalog *v3.Catalog, localPath string) string {
	if validCatalogKind[catalog.Spec.CatalogKind] {
		return catalog.Spec.CatalogKind
	}

	if _, err := os.Stat(filepath.Join(localPath, ".git", "HEAD")); !os.IsNotExist(err) {
		return KindHelmGit
	}

	pathURL := git.FormatURL(catalog.Spec.URL, catalog.Spec.Username, catalog.Spec.Password)
	if git.IsValid(pathURL) {
		return KindHelmGit
	}

	return KindHelmHTTP
}

func (h *Helm) Update(fetchLatest bool) (string, error) {
	h.lock()
	defer h.unlock()
	commit, err := h.update(fetchLatest)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(h.IconPath, 0755); err != nil {
		return "", err
	}
	return commit, nil
}

func (h *Helm) update(fetchLatest bool) (string, error) {
	logrus.Debugf("Helm preparing catalog cache [%s] for update", h.catalogName)
	if err := os.MkdirAll(h.LocalPath, 0755); err != nil {
		return "", err
	}

	var (
		commit string
		err    error
	)
	switch h.Kind {
	case KindHelmGit:
		commit, err = h.updateGit(fetchLatest)
	case KindHelmHTTP:
		commit, err = h.updateIndex(fetchLatest)
	case KindHelmInternal:
		commit, err = h.updateInternal()
	default:
		return "", fmt.Errorf("unknown helm catalog kind [%s] for [%s]", h.Kind, h.url)
	}
	return commit, err
}

func (h *Helm) updateInternal() (string, error) {
	empty, err := dirEmpty(h.LocalPath)
	if err != nil {
		return "", errors.Wrap(err, "empty directory check failed")
	}

	if empty {
		return "", errors.New("local catalog directory is empty")
	}

	commit, err := git.HeadCommit(h.LocalPath)
	if err != nil {
		err = errors.Wrap(err, "retrieving head commit failed")
	}

	return commit, err
}

func (h *Helm) updateIndex(fetchLatest bool) (string, error) {
	if !fetchLatest {
		return "", nil
	}

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

func (h *Helm) updateGit(fetchLatest bool) (string, error) {
	var (
		changed bool
		err     error
		commit  string
	)

	if h.branch == "" {
		h.branch = "master"
	}
	repoURL := git.FormatURL(h.url, h.username, h.password)

	empty, err := dirEmpty(h.LocalPath)
	if err != nil {
		return "", errors.Wrap(err, "Empty directory check failed")
	}

	if empty {
		if err = git.Clone(h.LocalPath, repoURL, h.branch); err != nil {
			return "", errors.Wrap(err, "Clone failed")
		}
	} else {
		if fetchLatest || h.lastCommit == "" {
			commit = fmt.Sprintf("origin/%s", h.branch)
			changed, err = remoteShaChanged(repoURL, h.branch, h.lastCommit, uuid)
			if err != nil {
				return "", errors.Wrap(err, "Remote commit check failed")
			}
		} else {
			commit = h.lastCommit
			changed, err = localShaDiffers(h.LocalPath, h.lastCommit)
			if err != nil {
				return "", errors.Wrap(err, "Local commit check failed")
			}
		}
		if changed {
			if err = git.Update(h.LocalPath, commit); err != nil {
				if h.username != "" && h.password != "" {
					// the error string contains url which is formed using url.UserInfo.String(),
					// which escapes the following characters-> '@', '/', '?' and ':'
					// h.password doesn't escape these characters, so to use strings.Contains(), obtain the escaped password
					userInfo := url.UserPassword(h.username, h.password)
					userPwd := strings.SplitN(userInfo.String(), ":", 2)
					if len(userPwd) != 2 {
						return "", errors.New("error getting userinfo url")
					}
					pwd := userPwd[1]
					if strings.Contains(err.Error(), pwd) {
						errStr := strings.Replace(err.Error(), pwd+"@", "***@", 1)
						err = errors.New(errStr)
					}
				}
				return "", errors.Wrap(err, "Update failed")
			}
			logrus.Debugf("Helm updated git repository for catalog [%s]", h.catalogName)
		}
	}

	commit, err = git.HeadCommit(h.LocalPath)
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

func localShaDiffers(localPath, commit string) (bool, error) {
	currentCommit, err := git.HeadCommit(localPath)
	return currentCommit != commit, err
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
	req.Header.Set("Accept", "application/vnd.github.v3.sha")
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
