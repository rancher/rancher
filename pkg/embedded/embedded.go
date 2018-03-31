package embedded

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/etcd/etcdmain"
	"github.com/rancher/rancher/pkg/clusteryaml"
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
		services.SchedulerContainerName,
		services.KubeControllerContainerName,
		services.EtcdContainerName,
	}
)

func Run(ctx context.Context) (context.Context, string, error) {
	rkeConfig, err := clusteryaml.LocalConfig()
	if err != nil {
		return ctx, "", err
	}

	bundle, err := rkecerts.Stage(rkeConfig)
	if err != nil {
		return ctx, "", err
	}

	plan, err := librke.New().GeneratePlan(ctx, rkeConfig)
	if err != nil {
		return ctx, "", err
	}

	processes := getProcesses(plan)
	eg, resultCtx := errgroup.WithContext(ctx)

	for name, process := range processes {
		runFn := func(ctx context.Context, args []string) {
			runK8s(ctx, bundle.KubeConfig(), args)
		}
		if name == "etcd" {
			runFn = runEtcd
		}
		eg.Go(runProcessFunc(ctx, name, process, runFn))
	}

	return resultCtx, bundle.KubeConfig(), nil
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

	hk := hyperkube.HyperKube{
		Name: "hyperkube",
		Long: "This is an all-in-one binary that can run any of the various Kubernetes servers.",
	}

	hk.AddServer(hyperkube.NewKubeAPIServer())
	hk.AddServer(hyperkube.NewKubeControllerManager())
	hk.AddServer(hyperkube.NewScheduler())

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
