package channelserver

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/flowcontrol"
)

var (
	prog             = "channelserver"
	channelserverCmd *exec.Cmd
	backoff          = flowcontrol.NewBackOff(5*time.Second, 15*time.Minute)
)

func GetURLAndInterval() (string, string) {
	val := map[string]interface{}{}
	if err := json.Unmarshal([]byte(settings.RkeMetadataConfig.Get()), &val); err != nil {
		logrus.Errorf("failed to parse %s value: %v", settings.RkeMetadataConfig.Name, err)
		return "", ""
	}
	url := data.Object(val).String("url")
	minutes, _ := strconv.Atoi(data.Object(val).String("refresh-interval-minutes"))
	if minutes <= 0 {
		minutes = 1440
	}

	return url, (time.Duration(minutes) * time.Minute).String()

}

func run() chan error {
	done := make(chan error, 1)
	go func() {
		defer close(done)

		url, interval := GetURLAndInterval()
		cmd := exec.Command(
			prog,
			"--config-key=k3s",
			"--url", url,
			"--url=/var/lib/rancher-data/driver-metadata/data.json",
			"--refresh-interval", interval,
			"--listen-address=0.0.0.0:8115",
			"--channel-server-version", getChannelServerArg(),
			getChannelServerArg())
		channelserverCmd = cmd
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		done <- cmd.Run()
	}()
	return done
}

func Start(ctx context.Context) error {
	if _, err := exec.LookPath(prog); err != nil {
		logrus.Errorf("Failed to find %s, will not run /v1-release API: %v", prog, err)
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			if err := Shutdown(); err != nil {
				logrus.Errorf("error terminating channelserver: %v", err)
			}
			return ctx.Err()
		case err := <-run():
			logrus.Infof("failed to run channelserver: %v", err)
		}
		backoff.Next("next", time.Now())
		select {
		case <-time.After(backoff.Get("next")):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// getChannelServerArg will return with an argument to pass to channel server
// to indicate the server version that is running. If the current version is
// not a proper release version, the argument will be empty.
func getChannelServerArg() string {
	serverVersion := settings.ServerVersion.Get()
	if !utils.ReleaseServerVersion(serverVersion) {
		return ""
	}
	return serverVersion
}

// Shutdown ends the channelserver process and resets backoff
func Shutdown() error {
	backoff.Reset("next")
	if channelserverCmd == nil {
		return nil
	}
	if err := channelserverCmd.Process.Kill(); err != nil {
		if !strings.Contains(err.Error(), "process already finished") {
			return err
		}
	}
	channelserverCmd = nil
	return nil
}
