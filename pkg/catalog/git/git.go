package git

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	catUtil "github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/wrangler/pkg/kv"
)

func Clone(path, url, branch string) error {
	if err := catUtil.ValidateURL(url); err != nil {
		return err
	}
	_, err := gogit.PlainClone(path, false, &gogit.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Progress:      os.Stdout,
	})
	return err
}

func Update(path, commit string) error {
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return err
	}
	if err := repo.Fetch(&gogit.FetchOptions{}); err != nil && err != gogit.NoErrAlreadyUpToDate {
		return err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	option := &gogit.CheckoutOptions{}
	if strings.Contains(commit, "/") {
		remote, branch := kv.Split(commit, "/")
		option.Branch = plumbing.NewRemoteReferenceName(remote, branch)
	} else {
		option.Hash = plumbing.NewHash(commit)
	}
	return worktree.Checkout(option)
}

func HeadCommit(path string) (string, error) {
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return "", err
	}
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}
	return ref.String(), nil
}

func RemoteBranchHeadCommit(url, branch string) (string, error) {
	if err := catUtil.ValidateURL(url); err != nil {
		return "", err
	}
	rem := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	refs, err := rem.List(&gogit.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, ref := range refs {
		return ref.Hash().String(), nil
	}
	return "", fmt.Errorf("no commit found for url %s branch %s", branch, url)
}

func IsValid(url string) bool {
	if err := catUtil.ValidateURL(url); err != nil {
		return false
	}
	rem := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	_, err := rem.List(&gogit.ListOptions{})
	return err == nil
}

// FormatURL generates request url if is a private catalog
func FormatURL(pathURL, username, password string) string {
	if len(username) > 0 && len(password) > 0 {
		if u, err := url.Parse(pathURL); err == nil {
			u.User = url.UserPassword(username, password)
			return u.String()
		}
	}
	return pathURL
}

func CloneWithDepth(path, url, branch string, depth int) error {
	if err := catUtil.ValidateURL(url); err != nil {
		return err
	}

	_, err := gogit.PlainClone(path, false, &gogit.CloneOptions{
		URL:           url,
		SingleBranch:  true,
		Depth:         depth,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Progress:      os.Stdout,
	})
	return err
}
