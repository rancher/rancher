package git

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/dynamiclistener/cert"
	cataloghttp "github.com/rancher/rancher/pkg/catalogv2/http"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/agent"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Credential        *corev1.Secret
	CABundle          []byte
	InsecureTLSVerify bool
}

func NewGit(directory, url string, opts *Options) (*Git, error) {
	if opts == nil {
		opts = &Options{}
	}

	g := &Git{
		url:               url,
		directory:         directory,
		caBundle:          opts.CABundle,
		insecureTLSVerify: opts.InsecureTLSVerify,
		secret:            opts.Credential,
	}
	return g, g.setCredential(opts.Credential)
}

type Git struct {
	url               string
	directory         string
	password          string
	agent             *agent.Agent
	caBundle          []byte
	insecureTLSVerify bool
	secret            *corev1.Secret
}

func (g *Git) setCredential(cred *corev1.Secret) error {
	if cred == nil {
		return nil
	}

	if cred.Type == corev1.SecretTypeBasicAuth {
		username, password := cred.Data[corev1.BasicAuthUsernameKey], cred.Data[corev1.BasicAuthPasswordKey]
		if len(password) == 0 && len(username) == 0 {
			return nil
		}

		u, err := url.Parse(g.url)
		if err != nil {
			return err
		}
		u.User = url.User(string(username))
		g.url = u.String()
		g.password = string(password)
	} else if cred.Type == corev1.SecretTypeSSHAuth {
		key, err := cert.ParsePrivateKeyPEM(cred.Data[corev1.SSHAuthPrivateKey])
		if err != nil {
			return err
		}
		sshAgent := agent.NewKeyring()
		err = sshAgent.Add(agent.AddedKey{
			PrivateKey: key,
		})
		if err != nil {
			return err
		}

		g.agent = &sshAgent
	}

	return nil
}

func (g *Git) clone(branch string) error {
	gitDir := filepath.Join(g.directory, ".git")
	if dir, err := os.Stat(gitDir); err == nil && dir.IsDir() {
		return nil
	}

	if err := os.RemoveAll(g.directory); err != nil {
		return err
	}

	return g.git("clone", "--depth=1", "-n", "--branch", branch, g.url, g.directory)
}

func (g *Git) needsUpdate(branch string) (string, bool, error) {
	if err := g.reset("HEAD"); err != nil {
		return "", true, err
	}

	commit, err := g.currentCommit()
	if err != nil {
		return commit, false, err
	}

	changed, err := g.remoteSHAChanged(branch, commit)
	return commit, changed, err
}

func (g *Git) Head(branch string) (string, error) {
	if err := g.clone(branch); err != nil {
		return "", nil
	}

	if err := g.reset("HEAD"); err != nil {
		return "", err
	}

	return g.currentCommit()
}

func (g *Git) Update(branch string) (string, error) {
	if err := g.clone(branch); err != nil {
		return "", nil
	}

	if err := g.reset("HEAD"); err != nil {
		return "", err
	}

	commit, err := g.currentCommit()
	if err != nil {
		return commit, err
	}

	if changed, err := g.remoteSHAChanged(branch, commit); err != nil || !changed {
		return commit, err
	}

	if err := g.fetchAndReset(branch); err != nil {
		return "", err
	}

	return g.currentCommit()
}

func (g *Git) fetchAndReset(rev string) error {
	if err := g.git("-C", g.directory, "fetch", "origin", rev); err != nil {
		return err
	}
	return g.reset("FETCH_HEAD")
}

func (g *Git) reset(rev string) error {
	return g.git("-C", g.directory, "reset", "--hard", rev)
}

func (g *Git) currentCommit() (string, error) {
	return g.gitOutput("-C", g.directory, "rev-parse", "HEAD")
}

func (g *Git) Ensure(commit string) error {
	if err := g.clone("master"); err != nil {
		return err
	}

	if err := g.reset(commit); err == nil {
		return nil
	}

	return g.fetchAndReset(commit)
}

func (g *Git) git(args ...string) error {
	return g.gitCmd(os.Stdout, args...)
}

func (g *Git) gitOutput(args ...string) (string, error) {
	output := &bytes.Buffer{}
	err := g.gitCmd(output, args...)
	return strings.TrimSpace(output.String()), err
}

func (g *Git) gitCmd(output io.Writer, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = output
	cmd.Stdin = bytes.NewBuffer([]byte(g.password))

	if g.agent != nil {
		c, err := g.injectAgent(cmd)
		if err != nil {
			return err
		}
		defer c.Close()
	}

	if g.insecureTLSVerify {
		cmd.Env = append(cmd.Env, "GIT_SSL_NO_VERIFY=false")
	}

	if len(g.caBundle) > 0 {
		f, err := ioutil.TempFile("", "ca-pem-")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		defer f.Close()

		if _, err := f.Write(g.caBundle); err != nil {
			return fmt.Errorf("writing cabundle to %s: %w", f.Name(), err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("closing cabundle %s: %w", f.Name(), err)
		}
		cmd.Env = append(cmd.Env, "GIT_SSL_CAINFO="+f.Name())
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("git %s error: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (g *Git) injectAgent(cmd *exec.Cmd) (io.Closer, error) {
	r, err := randomtoken.Generate()
	if err != nil {
	}

	addr := &net.UnixAddr{
		Name: "@ssh-agent-" + r,
		Net:  "unix",
	}

	l, err := net.ListenUnix(addr.Net, addr)
	if err != nil {
		return nil, err
	}

	cmd.Env = append(cmd.Env, "SSH_AUTH_SOCK="+addr.Name)

	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				logrus.Errorf("failed to accept ssh-agent client connection: %v", err)
				return
			}
			if err := agent.ServeAgent(*g.agent, conn); err != nil {
				logrus.Errorf("failed to handle ssh-agent client connection: %v", err)
			}
		}
	}()

	return l, nil
}

func (g *Git) remoteSHAChanged(branch, sha string) (bool, error) {
	formattedURL := formatGitURL(g.url, branch)
	if formattedURL == "" {
		return true, nil
	}

	client, err := cataloghttp.HelmClient(g.secret, g.caBundle, g.insecureTLSVerify)
	if err != nil {
		logrus.Warnf("Problem creating http client to check git remote sha of repo [%v]: %v", g.url, err)
		return true, nil
	}
	defer client.CloseIdleConnections()

	req, err := http.NewRequest("GET", formattedURL, nil)
	if err != nil {
		logrus.Warnf("Problem creating request to check git remote sha of repo [%v]: %v", g.url, err)
		return true, nil
	}

	req.Header.Set("Accept", "application/vnd.github.v3.sha")
	req.Header.Set("If-None-Match", fmt.Sprintf("\"%s\"", sha))
	if uuid := settings.InstallUUID.Get(); uuid != "" {
		req.Header.Set("X-Install-Uuid", uuid)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Return timeout errors so caller can decide whether or not to proceed with updating the repo
		uErr := &url.Error{}
		if ok := errors.As(err, &uErr); ok && uErr.Timeout() {
			return false, errors.Wrapf(uErr, "Repo [%v] is not accessible", g.url)
		}
		return true, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return false, nil
	}

	return true, nil
}

func formatGitURL(endpoint, branch string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}

	pathParts := strings.Split(u.Path, "/")
	switch u.Hostname() {
	case "github.com":
		if len(pathParts) >= 3 {
			org := pathParts[1]
			repo := strings.TrimSuffix(pathParts[2], ".git")
			return fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", org, repo, branch)
		}
	case "git.rancher.io":
		repo := strings.TrimSuffix(pathParts[1], ".git")
		u.Path = fmt.Sprintf("/repos/%s/commits/%s", repo, branch)
		return u.String()
	}

	return ""
}
