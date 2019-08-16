package telemetry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

var adminRole = "Default Admin"

type process struct {
	cmd     *exec.Cmd
	running bool
	lock    sync.RWMutex
}

func (p *process) kill() {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
}

func (p *process) getRunningState() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.running
}

func (p *process) setRunningState(state bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.running = state
}

func isLeader(management *config.ScaledContext) bool {
	if management.PeerManager == nil {
		return true
	}
	return management.PeerManager.IsLeader()
}

func Start(ctx context.Context, httpsPort int, management *config.ScaledContext) error {
	p := process{
		lock: sync.RWMutex{},
	}

	// have two go routines running. One is to run telemetry if setting is true, one is to kill telemetry if setting is false
	go func() {
		t := time.NewTicker(time.Second * 5)
		go func() {
			<-ctx.Done()
			t.Stop()
		}()
		defer t.Stop()
		for range t.C {
			if settings.TelemetryOpt.Get() == "in" && isLeader(management) {
				if !p.running {
					var token string
					var e error
					for i := 0; i < 3; i++ {
						token, e = createToken(management)
						if e != nil {
							logrus.Info(e)
							time.Sleep(time.Second * 5)
							continue
						} else {
							break
						}
					}
					if token == "" {
						logrus.Infof("Unable to obtain token for telemetry service. Telemetry will not be launched.")
						return
					}
					cmd := exec.Command("telemetry", "client", "--url", fmt.Sprintf("https://localhost:%d/v3", httpsPort), "--token-key", token)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					p.cmd = cmd
					if err := cmd.Start(); err != nil {
						logrus.Error(err)
						continue
					}
					p.setRunningState(true)
					// in here we don't care the error, it will block until the telemetry process exited
					cmd.Wait()
					p.setRunningState(false)
				}
			}
		}
	}()

	go func() {
		for range ticker.Context(ctx, time.Second*5) {
			if settings.TelemetryOpt.Get() != "in" || !isLeader(management) {
				if p.getRunningState() {
					p.kill()
					p.setRunningState(false)
				}
			}
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			if p.getRunningState() {
				p.kill()
				p.setRunningState(false)
			}
		}
	}()
	return nil
}

func createToken(management *config.ScaledContext) (string, error) {
	users, err := management.Management.Users("").Controller().Lister().List("", labels.SelectorFromSet(
		map[string]string{
			"authz.management.cattle.io/bootstrapping": "admin-user",
		},
	))
	if err != nil {
		return "", errors.Wrapf(err, "Can't list admin-user for telemetry. Err: %v. Retry after 5 seconds", err)
	}
	for _, user := range users {
		if user.DisplayName == adminRole {
			token, err := management.UserManager.EnsureToken("telemetry", "telemetry token", "telemetry", user.Name)
			if err != nil {
				return "", errors.Wrapf(err, "Can't create token for telemetry. Err: %v. Retry after 5 seconds", err)
			}
			return token, nil
		}
	}
	return "", errors.Errorf("user %s doesn't exist for telemetry. Retry after 5 seconds", adminRole)
}
