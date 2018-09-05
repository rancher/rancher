package embedded

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdmain"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/hyperkube"
	"github.com/rancher/rancher/pkg/k8scheck"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	copyProcesses = []string{
		services.KubeAPIContainerName,
		services.KubeControllerContainerName,
		services.EtcdContainerName,
	}
)

func Run(ctx context.Context) (context.Context, string, error) {
	rkeConfig, err := localConfig()
	if err != nil {
		return ctx, "", err
	}

	bundle, err := rkecerts.Stage(rkeConfig)
	if err != nil {
		return ctx, "", err
	}

	plan, err := librke.New().GeneratePlan(ctx, rkeConfig, nil)
	if err != nil {
		return ctx, "", err
	}

	processes := getProcesses(plan)
	eg, resultCtx := errgroup.WithContext(ctx)
	eg.Go(runProcessFunc(ctx, "etcd", processes["etcd"], runEtcd))

	if err := checkEtcd(bundle); err != nil {
		return ctx, "", errors.Wrap(err, "waiting on etcd")
	}

	for name, process := range processes {
		runFn := func(ctx context.Context, args []string) {
			runK8s(ctx, bundle.KubeConfig(), args)
		}
		if name == "etcd" {
			continue
		}
		eg.Go(runProcessFunc(ctx, name, process, runFn))
	}

	return resultCtx, bundle.KubeConfig(), nil
}

func checkEtcd(bundle *rkecerts.Bundle) error {
	certPool := x509.NewCertPool()
	certPool.AddCert(bundle.Certs()["kube-ca"].Certificate)

	ht := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
			Certificates: []tls.Certificate{
				{
					Certificate: [][]byte{
						bundle.Certs()["kube-etcd-127-0-0-1"].Certificate.Raw,
					},
					PrivateKey: bundle.Certs()["kube-etcd-127-0-0-1"].Key,
				}},
		},
	}
	client := http.Client{
		Transport: ht,
	}
	defer ht.CloseIdleConnections()

	for i := 0; ; i++ {
		resp, err := client.Get("https://localhost:2379/health")
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

func runProcessFunc(ctx context.Context, name string, process v3.Process, f func(context.Context, []string)) func() error {
	return func() error {
		runProcess(ctx, name, process, f)
		return fmt.Errorf("%s exited", name)
	}
}

func setEnv(env []string) {
	for _, part := range env {
		parts := strings.SplitN(part, "=", 2)
		if len(parts) == 1 {
			os.Setenv(parts[0], "")
		} else {
			os.Setenv(parts[0], parts[1])
		}
	}
}

func runEtcd(ctx context.Context, args []string) {
	os.Args = args
	logrus.Info("Running ", strings.Join(args, " "))
	etcdmain.Main()
	logrus.Errorf("etcd exited")
}

func runK8s(ctx context.Context, kubeConfig string, args []string) {
	if logrus.GetLevel() != logrus.DebugLevel {
		args = append(args, "-v=1")
	}
	args = append(args, "--logtostderr=false")
	args = append(args, "--alsologtostderr=false")

	if args[0] != "kube-apiserver" {
		restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			logrus.Errorf("Failed to build client: %v", err)
			return
		}
		if err := k8scheck.Wait(ctx, *restConfig); err != nil {
			logrus.Errorf("Failed to build client: %v", err)
			return
		}
	}

	if args[0] == "kube-controller-manager" {
		args = append(args, "--controllers", "*", "--controllers", "-resourcequota", "--controllers", "-service")
	}

	hk := hyperkube.HyperKube{
		Name: "hyperkube",
		Long: "This is an all-in-one binary that can run any of the various Kubernetes servers.",
	}

	hk.AddServer(hyperkube.NewKubeAPIServer())
	hk.AddServer(hyperkube.NewKubeControllerManager())

	logrus.Info("Running ", strings.Join(args, " "))
	if err := hk.Run(args, ctx.Done()); err != nil {
		logrus.Errorf("%s exited with error: %v", args[0], err)
	}
}

func runProcess(ctx context.Context, name string, p v3.Process, f func(context.Context, []string)) {
	env := append([]string{}, os.Environ()...)
	env = append(env, p.Env...)

	args := append([]string{}, p.Command...)
	args = append(args, p.Args...)
	for i, part := range args {
		if strings.HasPrefix(part, "-") {
			args = append([]string{name}, args[i:]...)
			break
		}
	}

	setEnv(env)
	f(ctx, args)
}

func getProcesses(plan v3.RKEPlan) map[string]v3.Process {
	processes := map[string]v3.Process{}
	for _, name := range copyProcesses {
		processes[name] = plan.Nodes[0].Processes[name]
	}
	return processes
}
