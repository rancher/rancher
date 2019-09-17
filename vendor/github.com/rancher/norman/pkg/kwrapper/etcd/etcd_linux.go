package etcd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func RunETCD(ctx context.Context, dataDir string) ([]string, error) {
	endpoint := "http://localhost:2379"

	go runEtcd(ctx, []string{"etcd", fmt.Sprintf("--data-dir=%s", filepath.Join(dataDir, "etcd"))})

	if err := checkEtcd(endpoint); err != nil {
		return nil, errors.Wrap(err, "waiting on etcd")
	}

	return []string{endpoint}, nil
}

func checkEtcd(endpoint string) error {
	ht := &http.Transport{}
	client := http.Client{
		Transport: ht,
	}
	defer ht.CloseIdleConnections()

	for i := 0; ; i++ {
		resp, err := client.Get(endpoint + "/health")
		if err != nil {
			if i > 1 {
				logrus.Infof("Waiting on etcd startup: %v", err)
			}
			time.Sleep(time.Second)
			continue
		}
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if i > 1 {
				logrus.Infof("Waiting on etcd startup: status %d", resp.StatusCode)
			}
			time.Sleep(time.Second)
			continue
		}

		break
	}

	return nil
}

func runEtcd(ctx context.Context, args []string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logrus.Info("Running ", strings.Join(args, " "))
	cmd.Run()
	logrus.Fatal("etcd exited")
}
