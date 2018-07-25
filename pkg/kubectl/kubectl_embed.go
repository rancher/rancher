// +build k8s

package kubectl

import (
	"os"
	"path/filepath"

	goflag "flag"

	"github.com/rancher/rancher/pkg/hyperkube"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
)

func Main() {
	hyperkubeCommand, allCommandFns := hyperkube.NewHyperKubeCommand()

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	basename := filepath.Base(os.Args[0])
	if err := hyperkube.CommandFor(basename, hyperkubeCommand, allCommandFns).Execute(); err != nil {
		logrus.Errorf("%s exited with error: %v", basename, err)
	}
}
